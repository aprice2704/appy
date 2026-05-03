// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 12
// :: description: Assembles HTML/JS/CSS constants for Appy.
// :: filename: code/cmd/appy/ui.go
// :: serialization: go

package main

const indexHTML = htmlTop + cssStyles + htmlMiddle + jsCore + jsRender + jsRetest + "\n// Initial state check\nsyncUIState();\n" + htmlBottom
