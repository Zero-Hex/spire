# EQ Model Viewer (vendored)

An unmodified copy of `index.html` (saved here as `viewer.html`) and `vendor/` from
https://github.com/Zero-Hex/Everquest_Model_Viewer at commit
`119c366f9236a940e0b5ccc9661cf6f135437dfd` (MIT, see LICENSE).

Spire's Icons & Models page loads it in a hidden frame to render previews
of imported item models: it calls the viewer's own `openArchiveFile`,
`openModel` and `exportIcon`, and catches the PNG by
replacing `saveFile`. To update, copy a newer `index.html` and `vendor/`
over these (renaming `index.html` to `viewer.html`: Spire's static file
handler redirects any `index.html` to its folder) and check that those
functions still exist.
