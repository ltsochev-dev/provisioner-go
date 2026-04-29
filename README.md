# Provisioner Go

Provisions k3s namespaces and tenants for Emo ERP.

## Configuration

| Variable            | Required | Default           | Description                                                               |
| ------------------- | -------- | ----------------- | ------------------------------------------------------------------------- |
| `DATABASE_URL`      | Yes      |                   | MySQL DSN used by the provisioner.                                        |
| `PROVISIONER_PORT`  | No       | `8181`            | HTTP port for the API server.                                             |
| `PROVISIONER_TOKEN` | No       | `dev-token`       | Bearer token for provisioner API calls.                                   |
| `KUBECONFIG`        | No       | client-go default | Path to kubeconfig. In-cluster config is used when running in Kubernetes. |
| `TENANT_APP_IMAGE`  | Yes      |                   | Docker image used for tenant Laravel pods and migration init containers.  |

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

## DBA Access

You'll need to provide full access to the database user to manage tenants and the global db

```sql
CREATE USER 'erp_provisioner'@'%' IDENTIFIED BY '...';

GRANT SELECT, INSERT, UPDATE, DELETE
ON erp_global.*
TO 'erp_provisioner'@'%';

GRANT CREATE
ON *.*
TO 'erp_provisioner'@'%';

GRANT CREATE USER
ON *.*
TO 'erp_provisioner'@'%';

GRANT ALL PRIVILEGES ON `tenant_%`.* TO 'erp_provisioner'@'%' WITH GRANT OPTION;
FLUSH PRIVILEGES;
```
