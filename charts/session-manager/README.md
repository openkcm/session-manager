# OpenKCM Session Manager Helm chart

This chart deploys [OpenKCM Session Manager](https://openkcm.io/components/cmk/session-manager),
which signs users of the CMK UI in with their tenant's identity provider. It runs
the OpenID Connect authorization code flow, keeps the sessions server-side in
Valkey, and stores the tenant trust records in PostgreSQL. ExtAuthZ checks every
browser request against Session Manager, and CMK maintains the trust records.

Every value is documented in [values.yaml](values.yaml), including every setting of
Session Manager itself under `config`, with an example of each alternative form. The
[Session Manager configuration reference](https://openkcm.io/components/cmk/security/session-manager-config)
describes these settings in more detail.
The [configuration guide](https://openkcm.io/components/cmk/security/configuration)
shows the complete setup with Envoy Gateway, ExtAuthZ and CMK.

## What the chart creates

| Resource | Purpose |
| --- | --- |
| Deployment `<release>-session-manager` | API server: the `/sm/*` sign-in endpoints (8080), the gRPC API for ExtAuthZ and CMK (9091), and the status endpoints (8888) |
| Deployment `<release>-session-manager-housekeeper` | One replica that deletes idle sessions and refreshes access tokens before they expire |
| Job `<release>-session-manager-migrate` | Creates and updates the database schema. Runs as a Helm hook before every install and upgrade |
| ConfigMap `<release>-session-manager` | The configuration, rendered from `config` and mounted as `/etc/session-manager/config.yaml` |
| ConfigMap `<release>-session-manager-config` | Hook copy of the configuration for the migration job. Kept until the next install or upgrade replaces it, also after `helm uninstall` |
| Service | `http`, `grpc` and `http-status` inside the cluster |
| ServiceAccount | Identity of the pods. Session Manager does not call the Kubernetes API, so no token is mounted |
| NetworkPolicy | Optional (`networkPolicy.enabled`): who may reach the pods |
| PodDisruptionBudget | Optional (`pod.disruptionBudget.enabled`), for the API server |
| HorizontalPodAutoscaler | Optional (`autoscaling.enabled`), for the API server |

## Prerequisites

- Kubernetes 1.23 or later (`autoscaling/v2`), and Helm 3 or later.
- PostgreSQL, with a database for Session Manager. The migration job creates the
  tables.
- Valkey. It holds the users' access and refresh tokens: protect it accordingly,
  preferably with mutual TLS.
- An OpenID Connect client registered at each tenant's identity provider, with
  the redirect URI `https://<API host>/sm/callback`.
- The CSRF secret that ExtAuthZ uses: both services must use exactly the same
  bytes.
- Envoy Gateway, routing `/sm/*` to the `http` port of the Service.

## Secrets you create

Keep every secret out of the values. Create Secrets, mount them with
`extraVolumes` and `extraVolumeMounts`, and refer to the files from `config` with
`source: file`:

| Secret content | Used by `config` key |
| --- | --- |
| Database password (and CA certificate, if `PGSSLROOTCERT` is used) | `database.password` |
| Valkey password, client certificate, key and CA certificate | `valkey.password`, `valkey.secretRef.mtls` |
| OpenID Connect client secret, or client certificate and key | `sessionManager.clientAuth.clientSecret`, `sessionManager.clientAuth.mTLS` |
| CSRF secret, **shared with ExtAuthZ**, at least 32 bytes | `sessionManager.csrfSecret` |

Write the files without a trailing newline: the files are read byte for byte.

## Install

```yaml
# session-manager-values.yaml
# image.tag defaults to the chart's appVersion; set it only to deploy another version.

replicaCount: 2

pod:
  disruptionBudget:
    enabled: true

networkPolicy:
  enabled: true
  http:
    from:
      - namespaceSelector: { matchLabels: { kubernetes.io/metadata.name: envoy-gateway-system } }
  grpc:
    from:
      - namespaceSelector: { matchLabels: { kubernetes.io/metadata.name: envoy-gateway-system } }
        podSelector: { matchLabels: { app.kubernetes.io/name: extauthz } }
      - podSelector: { matchLabels: { app.kubernetes.io/name: cmk } }

extraEnvs:
  - { name: PGSSLMODE, value: verify-full }
  - { name: PGSSLROOTCERT, value: /etc/credentials/database/ca.crt }

extraVolumes:
  - name: credentials
    projected:
      sources:
        - secret: { name: session-manager-database, items: [{ key: password, path: database/password }, { key: ca.crt, path: database/ca.crt }] }
        - secret: { name: session-manager-valkey, items: [{ key: password, path: valkey/password }, { key: tls.crt, path: valkey/tls.crt }, { key: tls.key, path: valkey/tls.key }, { key: ca.crt, path: valkey/ca.crt }] }
        - secret: { name: session-manager-oidc-client, items: [{ key: client-secret, path: client-auth/client-secret }] }
        - secret: { name: csrf-secret, items: [{ key: csrf_secret, path: csrf-secret/csrf_secret }] }
extraVolumeMounts:
  - { name: credentials, mountPath: /etc/credentials, readOnly: true }

config:
  database:
    host: { value: postgresql.services.svc.cluster.local }
    user: { value: session_manager }
    password: { source: file, file: { path: /etc/credentials/database/password, format: binary } }
  valkey:
    host: { value: valkey-primary.storage.svc.cluster.local:6379 }
    password: { source: file, file: { path: /etc/credentials/valkey/password, format: binary } }
    secretRef:
      type: mtls
      mtls:
        cert:     { source: file, file: { path: /etc/credentials/valkey/tls.crt, format: binary } }
        certKey:  { source: file, file: { path: /etc/credentials/valkey/tls.key, format: binary } }
        serverCa: { source: file, file: { path: /etc/credentials/valkey/ca.crt, format: binary } }
  sessionManager:
    callbackURL: https://api.example.com/sm/callback
    allowedRedirectBaseURLs:
      - https://ui.example.com/
    csrfSecret: { source: file, file: { path: /etc/credentials/csrf-secret/csrf_secret, format: binary } }
    clientAuth:
      type: client_secret
      clientID: cmk-session-manager
      clientSecret: { source: file, file: { path: /etc/credentials/client-auth/client-secret, format: binary } }
    csrfCookieTemplate:
      domain: example.com
  audit:
    endpoint: http://otel-collector.observability.svc:4318/v1/logs
```

```sh
helm upgrade --install session-manager ./charts/session-manager -n services -f session-manager-values.yaml
```

The release notes list the settings that are missing or not production-ready.

## Important values

| Value | Default | Description |
| --- | --- | --- |
| `image.tag` / `image.digest` | chart's `appVersion` | The Session Manager version to deploy. Leave both empty to deploy the version this chart release is made for |
| `image.repository` | `session-manager/images/session-manager` | Under `ghcr.io/openkcm` |
| `replicaCount` | `1` | Use two or more in production |
| `networkPolicy.enabled` | `false` | **Recommended.** The gRPC API has no authentication of its own |
| `config.database`, `config.valkey` | hosts and credentials empty | **Required** |
| `config.sessionManager.callbackURL` | empty | **Required.** Public URL of `/sm/callback` |
| `config.sessionManager.allowedRedirectBaseURLs` | `[]` | **Required.** Without it every sign-in fails with `400` |
| `config.sessionManager.csrfSecret` | empty | **Required.** The same secret as ExtAuthZ's `csrfSecret` |
| `config.sessionManager.clientAuth.clientID` | empty | **Required** at the first installation; the migration fails without it |
| `config.sessionManager.csrfCookieTemplate.domain` | empty | The parent domain of the UI and API hosts |
| `config.audit.endpoint` | not set | Where sign-in audit events are sent |
| `housekeeper.enabled` | `true` | Run exactly one housekeeper per installation |
| `migrate.enabled` | `true` | Run the database migration as a Helm hook |
| `extraVolumes`, `extraVolumeMounts`, `extraEnvs` | `[]` | Applied to all workloads, including the migration job |

## Security defaults

| Setting | Default |
| --- | --- |
| Pods | Non-root (user, group and `fsGroup` 65532), `RuntimeDefault` seccomp profile, no ServiceAccount token, no Service environment variables |
| Containers | No privilege escalation, all capabilities dropped, read-only root filesystem, writable `/tmp` |
| Secrets | No default values. Embedded values are reported in the release notes |
| gRPC reflection | Off |
| Profiling endpoints | Off |
| Identity providers over `http` | Refused (`config.sessionManager.allowHttpScheme`, and `allowHttpScheme` of the gRPC `session` service in `config.apps`, are false) |
| Valkey connection | **Without TLS** (`config.valkey.secretRef.type: insecure`). Use `mtls` unless a service mesh encrypts the traffic |
| PostgreSQL connection | `PGSSLMODE` not set: TLS if the server offers it, without a certificate check, else unencrypted. Set `PGSSLMODE=verify-full` and `PGSSLROOTCERT` in `extraEnvs` |

The gRPC API does not use TLS and does not authenticate its callers, and it can
change tenant trust records. Allow only ExtAuthZ and CMK to call it: enable
`networkPolicy`, or use the authorization policies of a service mesh with mutual
TLS.

## Network policy

With `networkPolicy.enabled: true`, the chart allows ingress to all pods of the
release only from the peers in `networkPolicy.http.from` (sign-in endpoints, for
Envoy Gateway), `networkPolicy.grpc.from` (gRPC API, for ExtAuthZ and CMK) and
`networkPolicy.status.from` (status port, for example a metrics collector). An
empty list denies all callers of that port. Outgoing traffic is not restricted.
Probes from the kubelet are not affected on most network plugins.

## Configuration changes

Session Manager reads its configuration and the referenced files at startup. The
chart adds a `checksum/config` annotation to the API server and housekeeper pods,
so a change under `config` restarts them during `helm upgrade`. Changes to the
Secrets you manage yourself need a restart:

```sh
kubectl rollout restart deployment/<release>-session-manager deployment/<release>-session-manager-housekeeper
```

Everything under `config` is written to `config.yaml` (helper `session-manager.config` in
[templates/_helpers.tpl](templates/_helpers.tpl)), for the API server, the housekeeper and
the migration job alike. `config.environment` and `config.labels` become
`application.environment` and `application.labels`, and `config.application` overrides
single application fields (the name defaults to `session-manager`, or `nameOverride` if
set). Session Manager ignores unknown keys without a warning, so check spelling and nesting
carefully.

## Database migrations

The migration job runs `session-manager migrate` before every install and
upgrade, with a hook copy of the configuration. The API server checks the schema
version at startup and does not start on an outdated schema. If the job fails,
the install or upgrade fails and the running pods are not changed. The job is
kept for `migrate.ttlSecondsAfterFinished` seconds (default 300), and its
ConfigMap until the next install or upgrade replaces it:

```sh
kubectl logs job/<release>-session-manager-migrate
```

The job does not get `extraContainers`, because a sidecar that keeps running
would keep it from completing. If a service mesh injects a sidecar, set
`migrate.podAnnotations` (for example `sidecar.istio.io/inject: "false"`).

## Upgrading from 0.10.x

| Change | Action |
| --- | --- |
| Default image repository is now `session-manager/images/session-manager` (was `images/session-manager`) | If you set `image.repository` yourself, nothing changes |
| No `imagePullSecrets` by default (was `registry-access`) | Set `imagePullSecrets` if you pull from a private registry |
| The ConfigMap is a regular release resource named `<release>-session-manager` (was the hook `<release>-session-manager-config`) | None. The migration job's hook copy replaces the old object under the same name and is kept, with the configuration of the latest install or upgrade, so pods rolled back to 0.10.x with `helm rollback` still find it. With `migrate.enabled: false`, the old ConfigMap stays unchanged; delete it once you no longer need a rollback |
| The configuration is always mounted as `/etc/session-manager/config.yaml` | `nameOverride` no longer breaks the installation |
| No default secrets: `csrfSecret`, `clientAuth.clientSecret`, `clientAuth.clientID`, the database and Valkey credentials and hosts are empty | Set them, preferably with `source: file` |
| `callbackURL` and `csrfCookieTemplate.domain` default to empty (were `localhost` values); `audit` is not set (was `http://localhost:4043/logs`) | Set them for your installation |
| `config.grpc.flags.reflection` defaults to `false` | Set it to `true` if you rely on reflection |
| Role and RoleBinding (without rules, in namespace `default`) are no longer created | None; Helm deletes them |
| New security defaults (user, group and `fsGroup` 65532, read-only root filesystem), probe thresholds, a TCP readiness probe of the API server on the `grpc` port, and resource requests | Override `pod.securityContext`, `securityContext`, the probes and `resources` if needed |
| The housekeeper has its own `housekeeper.resources`, probes and update strategy (a rolling update with `maxSurge: 0`, so the old pod stops before the new one starts), and gets `extraEnvs` and `extraEnvsFrom` | Move resource overrides for the housekeeper to `housekeeper.resources`. Do not set `housekeeper.strategy.type: Recreate`: with server-side apply (Helm 4) the upgrade fails |
| The migration job gets `extraEnvs`, `extraEnvsFrom`, `nodeSelector` and `tolerations`, but no longer `extraContainers` | Use `migrate.extraContainers` for containers that exit on their own |
| PodDisruptionBudget without `minAvailable` or `maxUnavailable` uses `maxUnavailable: 1` (was `minAvailable: 1`) | Set `minAvailable` explicitly if you want the old behaviour |
| `autoscaling.minReplicas` and `maxReplicas` default to 2 and 5 (were 1 and 1) | Set them explicitly if you enable autoscaling |
| `config.isImmutable` is replaced by `configMap.immutable` | The old key still works |
| The values for local development are no longer part of the published chart: `charts/session-manager/values-dev.yaml` in the session-manager repository is excluded from the package and used by the Makefile | Copy settings from `values.yaml` if you used them as a template |

## Troubleshooting

| Symptom | Cause |
| --- | --- |
| Install or upgrade fails in the migration job: `client_id is not set in the config` | Set `config.sessionManager.clientAuth.clientID` |
| Pods stay in `ContainerCreating` | A Secret or ConfigMap from `extraVolumes` is missing |
| API server exits with `DB schema version mismatch` | The migration job did not run: `migrate.enabled` is false, or the job failed |
| API server exits with `CSRF secret must be at least 32 bytes` | `config.sessionManager.csrfSecret` is missing or too short |
| API server exits with `creating valkey client` | Valkey not reachable, or `config.valkey.host` without port |
| Pods exit with `environment variable not set` | A value reference with `source: env` whose variable is not in `extraEnvs` |
| Sign-in fails with `400` | `allowedRedirectBaseURLs` is missing or does not match the UI's URL |
| Users can sign in, but API calls fail with `401` and ExtAuthZ logs `CSRF token is not valid` | Session Manager and ExtAuthZ use different CSRF secrets |

The [configuration reference](https://openkcm.io/components/cmk/security/session-manager-config#troubleshooting)
lists more symptoms.

## Uninstall

```sh
helm uninstall session-manager -n services
```

The Secrets you created yourself, the database and the Valkey data are not
removed. Helm does not remove hook resources either: delete the migration job's
ConfigMap yourself, and the migration job if `migrate.ttlSecondsAfterFinished`
has not removed it yet:

```sh
kubectl -n services delete configmap session-manager-config --ignore-not-found
kubectl -n services delete job session-manager-migrate --ignore-not-found
```
