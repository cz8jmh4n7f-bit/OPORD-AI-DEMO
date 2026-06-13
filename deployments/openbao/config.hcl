# OpenBao server config for the OPORD dev stack - persistent file storage so
# secrets survive restarts (the old `-dev` mode was in-memory), plus a
# plugin_directory so external secrets engines (e.g. the MPL GCP secrets engine)
# can be registered. Local-only: TLS is disabled here; a TLS listener + AppRole
# auth are separate prod-hardening steps.

storage "file" {
  path = "/openbao/file"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = true
}

plugin_directory = "/openbao/plugins"
disable_mlock    = true
api_addr         = "http://0.0.0.0:8200"
ui               = true
