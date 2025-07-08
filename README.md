# kubeclean

[![GitHub Release](https://img.shields.io/github/v/release/infrautils/kubeclean?style=flat-square)](https://github.com/infrautils/kubeclean/releases)
[![Container Image](https://img.shields.io/badge/image-ghcr.io%2Finfrautils%2Fkubeclean-blue?style=flat-square)](https://github.com/infrautils/kubeclean/pkgs/container/kubeclean)
[![License](https://img.shields.io/github/license/infrautils/kubeclean?style=flat-square)](LICENSE)

**kubeclean** is a lightweight Kubernetes-native controller that automatically cleans up Pods based on configurable rules such as Pod phase, TTL (Time-To-Live), namespaces, and label selectors. It helps maintain cluster hygiene by removing completed or failed Pods safely and automatically.

---

## 🚀 Features

- ✅ Automated cleanup of Pods in specific phases (e.g., `Succeeded`, `Failed`)
- ✅ Time-to-Live (TTL) based Pod cleanup rules
- ✅ Batch deletion support with customizable intervals
- ✅ Dry-run mode for safe testing before actual deletion
- ✅ Metrics and health endpoints for observability
- ✅ Optional secure TLS for metrics endpoints
- ✅ Easy deployment via Helm Chart & GitHub Container Registry (GHCR)

---

## 📦 Container Image

The official container image is hosted on **GitHub Container Registry (GHCR)**:

```bash
ghcr.io/infrautils/kubeclean:<version>
```

---

## 📥 Installation (Helm Chart via OCI Registry)

### 1. Login to the OCI Helm registry:
```bash
helm registry login ghcr.io
```

### 2. Pull and install the chart:
```bash
helm pull oci://ghcr.io/infrautils/charts/kubeclean --version <version>
helm install kubeclean ./kubeclean-<version>.tgz --namespace kubeclean --create-namespace
```

Alternatively, you can download the Helm chart from the [GitHub Releases](https://github.com/infrautils/kubeclean/releases) page.

---

## ⚙️ Configuration

Example `values.yaml`:

```yaml
replicaCount: 1

image:
  repository: ghcr.io/infrautils/kubeclean
  tag: "v1.2.3"
  pullPolicy: IfNotPresent

cleanup:
  interval: 2m
  config:
    dryRun: false
    batchSize: 10
    podCleanupConfig:
      enabled: true
      rules:
        - name: default-rule
          enabled: true
          ttl: "1h"
          phase: "Succeeded"
          namespaces: []
          selector: {}
```

### Key Configurations:
- **cleanup.interval**: Interval for batch cleanup runs (e.g., `2m`).
- **cleanup.config.dryRun**: If `true`, no actual deletion will occur (test mode).
- **podCleanupConfig.rules**: Define cleanup policies for Pods.

Other configurable sections:
- Resource limits (`resources`)
- Security contexts
- Node selectors, tolerations, and affinity
- TLS certificates for metrics endpoints

---

## 📈 Metrics & Health Probes

| Endpoint  | Port  | Path    |
|-----------|-------|---------|
| Metrics   | 8443  | `/metrics` |
| Health    | 8081  | `/healthz` and `/readyz` |

TLS can be enabled for metrics if needed.

---

## 🛠️ Release Workflow (Fully Automated)

- Container Image & Helm Chart versions are derived from Git tags (e.g., `v1.2.3`).
- Automated GitHub Actions:
  - Builds and pushes container image to `ghcr.io/infrautils/kubeclean`
  - Updates and packages Helm chart with matching version
  - Pushes Helm chart to `ghcr.io/infrautils/charts`
  - Generates GitHub Releases with release notes & install instructions.

---

## 👥 Contributing

We welcome contributions! To get started:

```bash
git clone https://github.com/infrautils/kubeclean.git
cd kubeclean
make build  # Adjust if you use a different build tool
```

> For major changes, please open an issue to discuss before submitting pull requests.

---

## 📄 License

This project is licensed under the [Apache 2.0 License](LICENSE).

---

## 📫 Support & Contact

For issues or suggestions, please [open an issue](https://github.com/infrautils/kubeclean/issues).

---

## 🙏 Credits
- Inspired by Kubernetes best practices.
- Maintained by the [infrautils](https://github.com/infrautils) community.
