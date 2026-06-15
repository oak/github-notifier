package assets

import _ "embed"

// Embedded icon files for the system tray

//go:embed icon.svg
var DarkIcon []byte

//go:embed icon_light.svg
var LightIcon []byte

//go:embed git-pull-request.svg
var GitPullRequestIcon []byte

//go:embed git-pull-request_light.svg
var GitPullRequestLightIcon []byte
