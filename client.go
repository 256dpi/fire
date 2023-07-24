package fire

import (
	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/xo"

	"github.com/256dpi/fire/coal"
)

// Client wraps a jsonapi.Client to directly interact with models.
type Client struct {
	client *jsonapi.Client
}

// NewClient will create and return a new client.
func NewClient(client *jsonapi.Client) *Client {
	return &Client{
		client: client,
	}
}

// List will list the provided models.
func (c *Client) List(model coal.Model, reqs ...jsonapi.Request) ([]coal.Model, *jsonapi.Document, error) {
	// list resources
	doc, err := c.client.List(c.getType(model), reqs...)
	if err != nil {
		return nil, doc, c.rewriteError(err)
	}

	// get listed models
	var listedModels []coal.Model
	for _, resource := range doc.Data.Many {
		model := coal.GetMeta(model).Make()
		err = AssignResource(model, resource)
		if err != nil {
			return nil, doc, err
		}
		listedModels = append(listedModels, model)
	}

	return listedModels, doc, nil
}

// Find will find and return the provided model.
func (c *Client) Find(model coal.Model, reqs ...jsonapi.Request) (coal.Model, *jsonapi.Document, error) {
	// find resource
	doc, err := c.client.Find(c.getType(model), model.ID().Hex(), reqs...)
	if err != nil {
		return nil, doc, c.rewriteError(err)
	}

	// get found model
	foundModel := coal.GetMeta(model).Make()
	err = AssignResource(foundModel, doc.Data.One)
	if err != nil {
		return nil, doc, err
	}

	return foundModel, doc, nil
}

// Create will create the provided model and return the created model.
func (c *Client) Create(model coal.Model) (coal.Model, *jsonapi.Document, error) {
	// convert model
	resource, err := ConvertModel(model)
	if err != nil {
		return nil, nil, err
	}

	// unset ID
	resource.ID = ""

	// create resource
	doc, err := c.client.Create(resource)
	if err != nil {
		return nil, doc, c.rewriteError(err)
	}

	// get created model
	createdModel := coal.GetMeta(model).Make()
	err = AssignResource(createdModel, doc.Data.One)
	if err != nil {
		return nil, doc, err
	}

	return createdModel, doc, nil
}

// Update will update the provided model and return the updated model.
func (c *Client) Update(model coal.Model) (coal.Model, *jsonapi.Document, error) {
	// convert model
	resource, err := ConvertModel(model)
	if err != nil {
		return nil, nil, err
	}

	// update resource
	doc, err := c.client.Update(resource)
	if err != nil {
		return nil, doc, c.rewriteError(err)
	}

	// get updated model
	updatedModel := coal.GetMeta(model).Make()
	err = AssignResource(updatedModel, doc.Data.One)
	if err != nil {
		return nil, doc, err
	}

	return updatedModel, doc, nil
}

// Delete will delete the provided model.
func (c *Client) Delete(model coal.Model) error {
	// delete resource
	err := c.client.Delete(c.getType(model), model.ID().Hex())
	if err != nil {
		return c.rewriteError(err)
	}

	return nil
}

func (c *Client) getType(model coal.Model) string {
	return coal.GetMeta(model).PluralName
}

func (c *Client) rewriteError(err error) error {
	// get error
	je, ok := err.(*jsonapi.Error)
	if !ok {
		return err
	}

	// check errors
	for _, e := range []xo.BaseErr{
		ErrAccessDenied,
		ErrResourceNotFound,
		ErrDocumentNotUnique,
	} {
		ee := e.Self().(*xo.Err).Err.(*jsonapi.Error)
		if ee.Status == je.Status && ee.Detail == je.Detail {
			return e.Self()
		}
	}

	return err
}

// ModelClient is model specific client.
type ModelClient[M coal.Model] struct {
	*Client
}

// ClientFor creates a model specific client for the specified model using the
// provided generic client.
func ClientFor[M coal.Model](c *Client) *ModelClient[M] {
	return &ModelClient[M]{
		Client: c,
	}
}

// List will return a list of models.
func (c *ModelClient[M]) List(reqs ...jsonapi.Request) ([]M, *jsonapi.Document, error) {
	// perform list
	var zero M
	models, doc, err := c.Client.List(zero, reqs...)
	if err != nil {
		return nil, doc, err
	}

	// convert models
	list := make([]M, 0, len(models))
	for _, m := range models {
		list = append(list, m.(M))
	}

	return list, doc, nil
}

// Find will find and return the model with the provided ID.
func (c *ModelClient[M]) Find(id coal.ID, reqs ...jsonapi.Request) (M, *jsonapi.Document, error) {
	// perform find
	var zero M
	model := coal.GetMeta(zero).Make().(M)
	model.GetBase().DocID = id
	m, doc, err := c.Client.Find(model, reqs...)
	if err != nil {
		return zero, doc, err
	}

	return m.(M), doc, nil
}

// Create will create the provided model and return the created model.
func (c *ModelClient[M]) Create(model M) (M, *jsonapi.Document, error) {
	// perform create
	m, doc, err := c.Client.Create(model)
	if err != nil {
		var zero M
		return zero, doc, err
	}

	return m.(M), doc, nil
}

// Update will update the provided model and return the updated model.
func (c *ModelClient[M]) Update(model M) (M, *jsonapi.Document, error) {
	// perform update
	m, doc, err := c.Client.Update(model)
	if err != nil {
		var zero M
		return zero, doc, err
	}

	return m.(M), doc, nil
}

// Delete will delete the model with the provided ID.
func (c *ModelClient[M]) Delete(id coal.ID) error {
	// perform delete
	var zero M
	model := coal.GetMeta(zero).Make().(M)
	model.GetBase().DocID = id
	err := c.Client.Delete(model)
	if err != nil {
		return err
	}

	return nil
}
