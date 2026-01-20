package cert

import "embed"

//go:embed CA.crt agent.crt agent.key client.crt client.key
var EmbedStore embed.FS
