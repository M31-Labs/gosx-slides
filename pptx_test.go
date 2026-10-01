package slides

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"io"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func TestPPTXPackageRelationshipsNotesAndImages(t *testing.T) {
	out := filepath.Join(t.TempDir(), "deck.pptx")
	writer, err := newPPTX(out)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.abort()
	var pixels bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 16, 9))
	img.Set(3, 3, color.White)
	png.Encode(&pixels, img)
	for i := 0; i < 2; i++ {
		if err := writer.add(pixels.Bytes(), "Title <&>", "Speaker <&> notes\nSecond line"); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.finish("Deck <&>"); err != nil {
		t.Fatal(err)
	}
	file, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	entries := map[string][]byte{}
	for _, f := range file.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		entries[f.Name], err = io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range entries {
		if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels") {
			decoder := xml.NewDecoder(bytes.NewReader(data))
			for {
				tok, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if start, ok := tok.(xml.StartElement); ok && start.Name.Local == "Relationship" {
					var target string
					for _, a := range start.Attr {
						if a.Name.Local == "Target" {
							target = a.Value
						}
					}
					base := path.Dir(name)
					if base == "_rels" {
						base = "."
					} else {
						base = path.Dir(base)
					}
					resolved := path.Clean(path.Join(base, target))
					if _, ok := entries[resolved]; !ok {
						t.Fatalf("%s: missing relationship target %s", name, resolved)
					}
				}
			}
		}
	}
	if !bytes.Equal(entries["ppt/media/slide1.png"], pixels.Bytes()) {
		t.Fatal("capture pixels changed")
	}
	if !bytes.Contains(entries["ppt/notesSlides/notesSlide2.xml"], []byte("Speaker &lt;&amp;&gt; notes")) {
		t.Fatal("notes lost")
	}
	if !bytes.Contains(entries["ppt/presentation.xml"], []byte(`cx="12192000" cy="6858000"`)) {
		t.Fatal("aspect changed")
	}
}
