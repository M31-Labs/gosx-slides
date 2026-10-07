# Web page slides

This deck embeds Example Domain and carries its recorded fallback into exports.

```sh
slides web refresh examples/webpage
slides serve examples/webpage
slides export examples/webpage --format single --out web-single
slides export examples/webpage --format pdf --out web.pdf
slides export examples/webpage --format pptx --out web.pptx
```

Snapshot refresh needs Chrome. Set `SLIDES_CHROME` if it is not on PATH.
Serving uses the included capture while the live page loads. All exports use
the captured image, URL and date. Add `offline-required: true` to the headmatter
to use only the recorded page even when serving.

`web-allow: [example.com]` permits only that exact HTTPS host. Scripts, origin
access and popups are disabled by default. The controls reload the active frame,
toggle fit/100% zoom, lock pointer interaction and open the URL separately.
Speaker previews use the image; their controls synchronize with the audience.

The PNG and `public/webpages/manifest.json` are deck assets. Refresh records a
new UTC date and content hash. Commit those assets when updating a capture.
See [the full authoring and export reference](../../README.md#web-page-slides).
