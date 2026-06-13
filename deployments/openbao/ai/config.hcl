# OpenBao server config for the ai-compose stack: PERSISTENT file storage (the
# old `-dev` mode was in-memory, so every provider key vanished on restart) and
# the web UI enabled. The openbao-init sidecar auto-initializes and auto-unseals,
# so operationally it still feels like dev mode - but secrets survive restarts.
# Local/test only: TLS is disabled; put a TLS listener + AppRole in front for prod.

storage "file" {
  path = "/openbao/file"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = true
}

disable_mlock = true
api_addr      = "http://openbao:8200"
ui            = true
