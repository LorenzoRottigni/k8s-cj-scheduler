# k8s-cj-scheduler

## Overview

The `k8s-cj-scheduler` project introduces a custom Kubernetes controller that simplifies the management of scheduled tasks within your cluster. Instead of directly interacting with Kubernetes `CronJob` resources, this operator allows you to define your schedules in a more declarative and streamlined way through a custom resource called `Scheduler`.

It's designed to abstract away the underlying `CronJob` complexities, enabling users to easily define multiple scheduled commands, specify container images, arguments, and even environment variables for each scheduled run, all within a single `Scheduler` object.

---

## Features

* **Declarative Scheduling**: Define recurring tasks using a custom `Scheduler` resource, making your scheduled workloads a first-class citizen in Kubernetes.
* **Multiple Schedules per Resource**: Consolidate multiple scheduled commands into a single `Scheduler` custom resource, simplifying management.
* **Customizable Container Images**: Specify any container image to run your scheduled commands.
* **Command-Line Arguments**: Pass custom arguments to your container commands via the `params` field.
* **Environment Variables**: Inject necessary environment variables into your scheduled jobs using the `env` field, supporting both literal values and dynamic values from the Downward API.
* **Automated CronJob Management**: The controller automatically creates, updates, and deletes Kubernetes `CronJob` resources based on your `Scheduler` definitions.
* **Cleanup**: Automatically removes `CronJob`s that are no longer defined in your `Scheduler` resource.

---

## Getting Started

To deploy and use the `k8s-cj-scheduler` operator in your Kubernetes cluster, follow these steps.

### Prerequisites

Before you begin, ensure you have the following installed:

* **Go**: `v1.24.0+`
* **Docker**: `v17.03+`
* **kubectl**: `v1.11.3+` (compatible with your cluster)
* **Kubernetes Cluster**: Access to a `v1.11.3+` Kubernetes cluster.

### Deploying to the Cluster

1.  **Build and Push Your Operator Image**:
    First, build your controller image and push it to a container registry you have access to. Replace `<your-registry>` with your registry's URL and `tag` with your preferred image tag (e.g., `v0.1.0`).

    ```sh
    make docker-build docker-push IMG=<your-registry>/k8s-cj-scheduler:tag
    ```
    > **Note**: Ensure the image is published to a registry that your Kubernetes cluster can pull from. If you encounter permission issues, check your registry credentials.

2.  **Install the Custom Resource Definitions (CRDs)**:
    This step registers the `Scheduler` custom resource with your Kubernetes API server.

    ```sh
    make install
    ```

3.  **Deploy the Operator (Manager)**:
    Deploy the `k8s-cj-scheduler` controller to your cluster. This will create the necessary deployments, service accounts, and RBAC roles.

    ```sh
    make deploy IMG=<your-registry>/k8s-cj-scheduler:tag
    ```
    > **Note**: If you face RBAC errors during deployment, you might need cluster-admin privileges or ensure your current `kubectl` context has sufficient permissions.

### Creating Your Scheduled Tasks

Once the controller is running, you can create instances of your `Scheduler` custom resource to define your jobs.

You can apply the provided sample configuration:

```sh
kubectl apply -k config/samples/
```

**Example `Scheduler` Resource (`config/samples/v1_scheduler.yaml` or `test.yml`):**

```yaml
apiVersion: rottigni.tech/v1 # Your API Group
kind: Scheduler
metadata:
  name: my-first-scheduler
  namespace: default
spec:
  schedules:
    - name: minutely-ping
      image: alpine/curl
      cronExpression: "*/1 * * * *" # Runs every minute
      params:
        - curl
        - "[https://www.google.com](https://www.google.com)"
    - name: env-variable-test
      image: busybox:latest
      cronExpression: "*/2 * * * *" # Runs every 2 minutes
      params:
        - sh
        - -c
        - "echo 'Hello from ENVIRONMENT!'; echo 'MY_CUSTOM_VAR is: $MY_CUSTOM_VAR'; echo 'ANOTHER_SECRET_VAR is: $ANOTHER_SECRET_VAR'; sleep 5"
      env:
        - name: MY_CUSTOM_VAR
          value: "This is a custom value!"
        - name: ANOTHER_SECRET_VAR
          value: "Shhh, this is a secret!"
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
```
> **Note**: After applying, the `k8s-cj-scheduler` controller will create corresponding `CronJob` resources in the `default` namespace. You can verify them with `kubectl get cronjobs`.

---

## Usage Example

Let's assume you've deployed the `k8s-cj-scheduler` operator to your cluster. Here's a quick example of how you can define and manage a scheduled task.

1.  **Define Your `Scheduler` Resource**:
    Create a file named `my-daily-report-scheduler.yaml` with the following content:

    ```yaml
    apiVersion: rottigni.tech/v1
    kind: Scheduler
    metadata:
      name: daily-report-generator
      namespace: default # Or your desired namespace
    spec:
      schedules:
        - name: generate-report
          image: myorg/report-generator:latest # Replace with your actual image
          cronExpression: "0 0 * * *" # Runs every day at midnight UTC
          params:
            - "/app/generate-report.sh"
            - "--date=$(date +%Y-%m-%d)"
          env:
            - name: REPORT_TYPE
              value: "daily-summary"
            - name: API_KEY_SECRET_NAME
              valueFrom:
                secretKeyRef:
                  name: my-api-key-secret # Name of your secret
                  key: api_key
    ```
    This `Scheduler` resource will create a `CronJob` that runs daily at midnight, executing a report generation script with specific parameters and environment variables (including one fetched from a Kubernetes Secret).

2.  **Apply the `Scheduler` Resource**:
    ```sh
    kubectl apply -f my-daily-report-scheduler.yaml
    ```

3.  **Verify the Created CronJob**:
    You can see the `CronJob` created by your operator:
    ```sh
    kubectl get cronjobs -n default
    # Expected output similar to:
    # NAME                       SCHEDULE        SUSPEND   FORBIDDEN   AGE
    # daily-report-generator-generate-report   0 0 * * * False     False       10s
    ```

4.  **Inspect Job Runs (Optional)**:
    As time passes, `CronJob`s will create `Job` resources. You can check them with:
    ```sh
    kubectl get jobs -n default -l scheduler=daily-report-generator
    ```
    And view logs of a specific job run:
    ```sh
    kubectl logs -f <job-name-from-above-command> -n default
    ```

---

## Uninstallation

To remove the `k8s-cj-scheduler` and all its components from your cluster:

1.  **Delete Your `Scheduler` Instances**:
    This will trigger the controller to clean up all associated `CronJob`s.

    ```sh
    kubectl delete -f my-daily-report-scheduler.yaml # Delete your specific example
    kubectl delete -k config/samples/ # If you used the sample kustomize base
    ```

2.  **Delete the CRDs**:
    This removes the `Scheduler` API from your cluster.

    ```sh
    make uninstall
    ```

3.  **Undeploy the Controller Manager**:
    This removes the operator's deployment and related resources.

    ```sh
    make undeploy
    ```

---

## Project Distribution

If you wish to package and distribute your `k8s-cj-scheduler` solution to others:

### 1. Providing a YAML Bundle

You can generate a single YAML file containing all necessary Kubernetes resources for installation:

1.  **Build the Installer Bundle**:

    ```sh
    make build-installer IMG=<your-registry>/k8s-cj-scheduler:tag
    ```
    This command generates an `install.yaml` file in the `dist` directory. This file bundles all resources required to install the project.

2.  **Distribute and Use**:
    Users can then install the project by simply applying this bundle:

    ```sh
    kubectl apply -f [https://raw.githubusercontent.com/](https://raw.githubusercontent.com/)<your-org>/k8s-cj-scheduler/<tag-or-branch>/dist/install.yaml
    ```
    (Replace `<your-org>` and `<tag-or-branch>` with your actual GitHub organization and desired release tag/branch).

### 2. Providing a Helm Chart

You can leverage the optional Helm plugin to generate a Helm Chart for easier deployment via Helm.

1.  **Generate/Update the Helm Chart**:
    ```sh
    kubebuilder edit --plugins=helm/v1-alpha
    ```
    This command generates (or updates) a Helm chart under `dist/chart`.

2.  **Distribute the Chart**:
    Users can obtain and install your solution using standard Helm commands.

    > **Note**: If you modify your project (e.g., add webhooks), you'll need to re-run the above `kubebuilder edit` command to sync changes to the Helm Chart. Use the `--force` flag if necessary, and manually re-apply any custom configurations in `dist/chart/values.yaml` or `dist/chart/manager/manager.yaml`.

---

## Contributing

We welcome contributions to the `k8s-cj-scheduler` project! To contribute:

1.  **Fork** the repository.
2.  **Clone** your forked repository: `git clone https://github.com/<your-username>/k8s-cj-scheduler.git`
3.  Create a new **feature branch**: `git checkout -b feature/your-feature-name`
4.  Make your changes and **commit** them: `git commit -m "feat: Add new feature"`
5.  **Push** to your branch: `git push origin feature/your-feature-name`
6.  Open a **Pull Request** to the `main` branch of the upstream repository.

Please ensure your code adheres to the existing style and conventions. Run `make test` and `make fmt` before submitting.

---

**Additional Information**: Run `make help` for more information on all potential `make` targets.

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html).

---

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.