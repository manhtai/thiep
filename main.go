package main

import (
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func loadFont(fontPath string, fontSize float64) (font.Face, error) {
	fontBytes, err := tpl.ReadFile(fontPath)
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

func generateInvite(templatePath, fontPath, text string) (*image.RGBA, error) {
	file, err := os.Open(templatePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	// Convert the image to RGBA format
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, img.Bounds(), img, image.Point{}, draw.Src)

	// Load custom font
	fontSize := 32.0
	face, err := loadFont(fontPath, fontSize)
	if err != nil {
		return nil, err
	}
	defer face.Close() // Remember to close the font face when done

	// Specify the position to insert the text
	x, y := 1560-measureTextWidth(text, face)/2, 407

	// Add the text to the image
	addLabel(rgba, x, y, text, face)

	return rgba, nil
}

//go:embed tpl/*
var tpl embed.FS

func serveImage(tp, text string, w http.ResponseWriter, r *http.Request) {
	img, err := generateInvite(fmt.Sprintf("tpl/%s.jpg", tp), "tpl/font.ttf", text)
	if err != nil {
		log.Print(err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "max-age=3600")
	w.WriteHeader(http.StatusOK)
	jpeg.Encode(w, img, nil)
}

type RenderData struct {
	Text    string
	Tpl     string
	ImgHash string
	Ok      bool
	Host    string
}

func main() {
	router := http.NewServeMux()
	host := os.Getenv("HOST")

	router.HandleFunc("GET /tao", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFS(tpl, "tpl/index.html")
		if err != nil {
			log.Print(err)
			return
		}

		tp := r.URL.Query().Get("tpl")
		text := r.URL.Query().Get("text")
		hash := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf(`%s/%s`, tp, text)))
		ok := slices.Contains([]string{"th", "vq"}, tp) && text != ""
		tmpl.Execute(w, RenderData{
			ImgHash: hash,
			Text:    text,
			Tpl:     tp,
			Ok:      ok,
			Host:    host,
		})
	})

	router.HandleFunc("GET /t/{code}", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFS(tpl, "tpl/page.html")
		if err != nil {
			log.Print(err)
			return
		}

		code := r.PathValue("code")
		data, err := base64.URLEncoding.DecodeString(code)
		if err != nil {
			log.Print(err)
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		paths := strings.Split(string(data), "/")
		if len(paths) < 2 {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		tp, text := paths[0], paths[1]
		hash := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf(`%s/%s`, tp, text)))
		ok := slices.Contains([]string{"th", "vq"}, tp) && text != ""

		tmpl.Execute(w, RenderData{
			ImgHash: hash,
			Text:    text,
			Tpl:     tp,
			Ok:      ok,
			Host:    host,
		})
	})

	router.HandleFunc("GET /i/{code}", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		data, err := base64.URLEncoding.DecodeString(code)
		if err != nil {
			log.Print(err)
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		paths := strings.Split(string(data), "/")
		if len(paths) < 2 {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		tp, text := paths[0], paths[1]
		log.Print("tp", tp, "text", text)
		serveImage(tp, text, w, r)
	})

	router.HandleFunc("GET /to/{tp}/{text}", func(w http.ResponseWriter, r *http.Request) {
		tp := r.PathValue("tp")
		text := r.PathValue("text")
		serveImage(tp, text, w, r)
	})

	router.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://huyentrang.manhtai.com", http.StatusTemporaryRedirect)
	})

	router.HandleFunc("GET /{file}", func(w http.ResponseWriter, r *http.Request) {
		file := r.PathValue("file")
		http.ServeFileFS(w, r, tpl, fmt.Sprintf("tpl/%s", file))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), router)
	if err != nil {
		log.Fatal(err)
	}
}
