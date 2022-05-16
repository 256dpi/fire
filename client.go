package fire

import (
	"github.com/256dpi/jsonapi/v2"

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
	doc, err := c.client.List(getType(model), reqs...)
	if err != nil {
		return nil, doc, err
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
	doc, err := c.client.Find(getType(model), model.ID().Hex(), reqs...)
	if err != nil {
		return nil, doc, err
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

	// unset id
	resource.ID = ""

	// create resource
	doc, err := c.client.Create(resource)
	if err != nil {
		return nil, doc, err
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
		return nil, doc, err
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
	err := c.client.Delete(getType(model), model.ID().Hex())
	if err != nil {
		return err
	}

	return nil
}

func getType(model coal.Model) string {
	return coal.GetMeta(model).PluralName
}
