// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 13
// :: description: Assembles HTML/JS/CSS constants for Appy.
// :: filename: ui.go
// :: serialization: go

package main

const indexHTML = htmlTop + cssStyles + htmlMiddle + jsCore + jsRender + jsRetest + "\n// Initial state check\nsyncUIState();\n" + htmlBottom
