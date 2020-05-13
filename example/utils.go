package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand"
)

func randomImage() *bytes.Buffer {
	// prepare image
	img := image.NewRGBA(image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: 512, Y: 512},
	})

	// generate colors
	c1 := color.RGBA{
		R: uint8(rand.Intn(256)),
		G: uint8(rand.Intn(256)),
		B: uint8(rand.Intn(256)),
		A: 255,
	}
	c2 := color.RGBA{
		R: uint8(rand.Intn(256)),
		G: uint8(rand.Intn(256)),
		B: uint8(rand.Intn(256)),
		A: 255,
	}

	// draw image
	for x := 0; x < 512; x++ {
		for y := 0; y < 512; y++ {
			switch {
			case x < 256 && y < 256 || x > 256 && y > 256:
				img.Set(x, y, c1)
			default:
				img.Set(x, y, c2)
			}
		}
	}

	// encode image
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		panic(err.Error())
	}

	return &buf
}
