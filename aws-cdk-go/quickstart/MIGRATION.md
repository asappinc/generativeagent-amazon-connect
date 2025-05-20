# Migration Guide: 1.x → 2.x

This guide explains how to migrate your ASAPP GenerativeAgent Amazon Connect deployment from version 1.x to 2.x.

---

## Key Changes in 2.x

- **Redis is replaced by Valkey:** The stack now provisions a Valkey replication group instead of Redis.
- **Lambda functions:** Lambdas now use the `@valkey/valkey-glide` client for Redis operations.
- **Configuration:** The configuration file must include a new `valkeyParameters` section.
- **Docker requirement:** Docker is now required for packaging Lambda functions with native dependencies. 

---

## Migration Steps

1. **Destroy the existing 1.x stack**

   Refer to the section [Destroy the CDK stack](./README.md#destroy-the-cdk-stack) for instructions on how to remove all resources created by the previous version.  
   > **Note:** This will delete all AWS resources provisioned by the previous stack, including Redis.

2. **Download or clone the 2.x version**

    If you haven't already, clone this repository or download it using the provided repository URL.  
    Otherwise, if you already have a local copy, pull the latest changes to ensure you have the 2.x version. You can check your current version in the `CHANGELOG.md` file at the root of the repository.

3. **Update your configuration file**

    Edit your existing configuration file (e.g., `config.<envName>.json`) and add the new `valkeyParameters` section. 

    Example:

    ```json
    "valkeyParameters": {
        "cacheNodeType": "cache.t4g.micro",
        "replicaNodesCount": 1
    }
    ```
       
    For details on how the configuration file should look, refer to the [2. ### Complete the configuration file](./README.md#complete-the-configuration-file) section of the README.md.

4. **Deploy the 2.x stack**

Follow the instructions in the [Deploy the CDK stack](./README.md#deploy-the-cdk-stack) section of the README.md to set up and deploy the new stack.
 > **Note:** New Requirement: Docker must be installed and running on your machine to package Lambda functions with native dependencies.
