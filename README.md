# Provisioner Go

Provisions k3s namespaces and tenants for Emo ERP.

## Configuration

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `DATABASE_URL` | Yes |  | MySQL DSN used by the provisioner. |
| `PROVISIONER_PORT` | No | `8181` | HTTP port for the API server. |
| `PROVISIONER_TOKEN` | No | `dev-token` | Bearer token for provisioner API calls. |

## Development

```sh
make fmt
make test
make vet
make run
```

## API

- `GET /health`
- `POST /tenants`
- `GET /tenants/{slug}`
