# Installation

This document provides detailed instructions for installing `access-log-exporter`.

## Installing via Linux Packages

DEB/RPM packages for Linux distributions exist. You can download the latest package from the [release page](https://github.com/jkroepke/access-log-exporter/releases).

### For Debian-based distributions:

`access-log-exporter` provides an APT repository for Debian-based distributions. Add the repository and install the package using the following commands:

```bash
curl -L https://raw.githubusercontent.com/jkroepke/access-log-exporter/refs/heads/main/packaging/apt/access-log-exporter.sources | sudo tee /etc/apt/sources.list.d/access-log-exporter.sources
sudo apt update
sudo apt install access-log-exporter
```

Note: The APT repository contains only the latest release.
To pin a specific version, use `https://github.com/jkroepke/access-log-exporter/releases/download/vX.Y.Z` as URIs in the sources file.

**Alternatively, you can install the DEB package manually:**

1. Download the DEB package from the releases page.
2. Open a terminal.
3. Navigate to the directory where you downloaded the package.
4. Install the package using the following command:

```bash
sudo dpkg -i <package_file>.deb
```

Replace `<package_file>` with the name of the downloaded file.

### For RedHat-based distributions:

1. Download the RPM package from the releases page.
2. Open a terminal.
3. Navigate to the directory where you downloaded the package.
4. Install the package using the following command:

```bash
sudo yum localinstall <package_file>.rpm
```

Replace `<package_file>` with the name of the downloaded file.

## Running as Kubernetes Sidecar

When using Kubernetes, you can run `access-log-exporter` as a sidecar container in your pod. This allows it to access the logs of your main application container.

To do this, add the following configuration to your pod's YAML file. The configuration varies slightly depending on your Kubernetes version.

### Kubernetes 1.33 and higher: Run as a sidecar container

<details>

<summary>Click to expand</summary>

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      initContainers:
      - name: access-log-exporter
        image: ghcr.io/jkroepke/access-log-exporter:latest
        ports:
          - containerPort: 4040
            name: metrics
          - containerPort: 8514
            name: syslog
        restartPolicy: Always
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop: ["all"]
          privileged: false
          runAsNonRoot: true
          runAsUser: 65532
          runAsGroup: 65532

      containers:
      - name: nginx
        image: nginx:latest
```

</details>

For more information on how to configure the sidecar container, refer to the [Kubernetes documentation](https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/).

### Kubernetes 1.32 and lower: Run as an extra container

<details>

<summary>Click to expand</summary>

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:latest

      - name: access-log-exporter
        image: ghcr.io/jkroepke/access-log-exporter:latest
        ports:
          - containerPort: 4040
            name: metrics
          - containerPort: 8514
            name: syslog
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop: ["all"]
          privileged: false
          runAsNonRoot: true
          runAsUser: 65532
          runAsGroup: 65532
```

</details>

## Manual Installation

To build the binary yourself, follow these steps:
1. Ensure you have [Go](https://go.dev/doc/install) and Make installed on your system.
2. Download the source code from our [releases page](https://github.com/jkroepke/access-log-exporter/releases/latest).
3. Open a terminal.
4. Navigate to the directory where you downloaded the source code.
5. Build the binary using the following command:
    ```bash
    make build
    ```
    This creates a binary file named access-log-exporter.
6. Move the `access-log-exporter` binary to /usr/bin/ using the following command:
    ```bash
    sudo mv access-log-exporter /usr/bin/
    ```

7. Verify the installation by checking the version:
    ```bash
    access-log-exporter --version
    ```

Continue with the [Configuration Guide](./Configuration.md) to set up your provider details.
