package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"net/http"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func loadFont(fontPath string, fontSize float64) (font.Face, error) {
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("could not read font file: %v", err)
	}

	parsedFont, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("could not parse font: %v", err)
	}

	const dpi = 72
	face, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create font face: %v", err)
	}

	return face, nil
}

func measureTextWidth(text string, face font.Face) int {
	var width fixed.Int26_6
	for _, char := range text {
		advance, ok := face.GlyphAdvance(char)
		if ok {
			width += advance
		}
	}
	return width.Round()
}

func addLabel(img *image.RGBA, x, y int, label string, face font.Face) {
	col := color.RGBA{R: 255, A: 255} // Red color for the text
	d := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(label)
}

func generateInvite(templatePath, fontPath, text string, writer io.Writer) error {
	file, err := os.Open(templatePath)
	if err != nil {
		return err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return err
	}

	// Convert the image to RGBA format
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, img.Bounds(), img, image.Point{}, draw.Src)

	// Load custom font
	fontSize := 32.0
	face, err := loadFont(fontPath, fontSize)
	if err != nil {
		return err
	}
	defer face.Close() // Remember to close the font face when done

	// Specify the position to insert the text
	x, y := 1560-measureTextWidth(text, face)/2, 407

	// Add the text to the image
	addLabel(rgba, x, y, text, face)

	return jpeg.Encode(writer, rgba, nil)
}

func main() {
	router := http.NewServeMux()
	router.HandleFunc("GET /{tpl}/{text}", func(w http.ResponseWriter, r *http.Request) {
		tpl := r.PathValue("tpl")
		text := r.PathValue("text")
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.WriteHeader(http.StatusOK)
		generateInvite(fmt.Sprintf("./%s.jpg", tpl), "./PlaywriteHRLijeva-VariableFont_wght.ttf", text, w)
	})

	http.ListenAndServe(":3001", router)
}
