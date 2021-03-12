# Client-based Secret Retrieval for Kubernetes

In the new DevOps world, many enterprises have started to shift their development process on to the Cloud Native platform where applications are built in containers that are managed/orchestrated by Kubernetes. Today we can configure our current Privileged Access Service and Centrify Client offering to solve the application secret problems inside containers using mutating admission webhook.

## Requirements

- [Go](https://golang.org/doc/install) 1.13 or higher
- [Docker Desktop](https://www.docker.com/products/docker-desktop) with Kubernetes enabled

## Build

1. Build binaries

```sh
$ make build
```

2. Build Docker images

```sh
$ make image
```

## Deploy Kubernetes Webhook

1. Create a signed cert/key pair and store it in a Kubernetes secret that will be consumed by webhook deployment

```sh
$ ./scripts/webhook-create-signed-cert.sh
```

2. Patch the MutatingWebhookConfiguration by replacing ${caBundle} in template file mutatingwebhook.template.v1beta1 with correct value from Kubernetes cluster. Following command creates a new deployment file from template.

```sh
$ cat deployment/mutatingwebhook.template.v1beta1 | \
    scripts/webhook-patch-ca-bundle.sh > \
    deployment/mutatingwebhook.yaml
```

3. Deploy Webhook. Following make command creates a Deployment, Service and MutatingWebhookConfiguration.

```sh
$ make deploy
```

4. Verify webhook is deployed successfully

```sh
$ kubectl get all
NAME                                             READY   STATUS    RESTARTS   AGE
pod/webhook-server-deployment-77d598dc67-cck8g   1/1     Running   0          3m21s

NAME                         TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)        AGE
service/kubernetes           ClusterIP   10.96.0.1        <none>        443/TCP        56d
service/webhook-server-svc   ClusterIP   10.107.125.182   <none>        443/TCP        3m21s

NAME                                        READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/webhook-server-deployment   1/1     1            1           3m21s

NAME                                                   DESIRED   CURRENT   READY   AGE
replicaset.apps/webhook-server-deployment-77d598dc67   1         1         1       3m21s
```

5. ADD OAuth2 token
   If OAuth2 authentication is used by init container, first obtain OAuth2 token from Centrify tenant then store it in Kubernetes secret. **Note** secret name must match the vaule of vault.centrify.com/oauth-secret-name annotation.

```sh
$ kubectl create secret generic vault-token --from-literal='oauthtoken=REPLACE OAUTH2 TOKEN HERE'
```


## Deploy Application

To deploy sample [WordPress](https://kubernetes.io/docs/tutorials/stateful-application/mysql-wordpress-persistent-volume/) application using init container injection and sidecar container injection methods, download and modify mysql-deployment.yaml and wordpress-deployment.yaml manifest files from above web site to add appropriate annotiations. Sample files are provided in deployment directory. wordpress_mysql-deployment.yaml uses init container injection while wordpress_web-deployment.yaml uses sidecar container injection.

```sh
$ kubectl apply -f deployment/wordpress_mysql-deployment.yaml
$ kubectl apply -f deployment/wordpress_web-deployment.yaml
```

Verify WordPress application is deployed successfully.

```sh
$ kubectl get all
NAME                                             READY   STATUS    RESTARTS   AGE
pod/webhook-server-deployment-77d598dc67-lprpr   1/1     Running   0          35m
pod/wordpress-656c879574-b4b5k                   2/2     Running   0          44s
pod/wordpress-mysql-96dc54c5c-wwktb              1/1     Running   0          18m

NAME                         TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)        AGE
service/kubernetes           ClusterIP      10.96.0.1      <none>        443/TCP        56d
service/webhook-server-svc   ClusterIP      10.111.97.93   <none>        443/TCP        35m
service/wordpress            LoadBalancer   10.106.43.67   localhost     80:32359/TCP   44s
service/wordpress-mysql      ClusterIP      None           <none>        3306/TCP       18m
service/wordpress-np         NodePort       10.96.87.242   <none>        80:31929/TCP   36d

NAME                                        READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/webhook-server-deployment   1/1     1            1           35m
deployment.apps/wordpress                   1/1     1            1           44s
deployment.apps/wordpress-mysql             1/1     1            1           18m

NAME                                                   DESIRED   CURRENT   READY   AGE
replicaset.apps/webhook-server-deployment-77d598dc67   1         1         1       35m
replicaset.apps/wordpress-656c879574                   1         1         1       44s
replicaset.apps/wordpress-mysql-96dc54c5c              1         1         1       18m
```


### Verify Deployment

Notice pod/wordpress-656c879574-b4b5k pod contains 2 containers and one of them is injected sidecar container.

Check that password file is injected into wordpress container volume. Replace pod name corresponds to your deployment in kubectl command.
```sh
$ kubectl exec --stdin --tty pod/wordpress-656c879574-b4b5k --container wordpress -- ls /centrify/secrets
WORDPRESS_DB_PASSWORD
```

Check that sidecar container is joined to Centrify tenant. Replace pod name corresponds to your deployment in kubectl command.
```sh
$ kubectl exec --stdin --tty pod/wordpress-656c879574-b4b5k --container centrifyk8s-sidecar -- cinfo
Enrolled in:       https://abc0751.my.centrify.net/
Enrolled as:
    Service account:  wordpress-656c879574-b4b5k$@centrify.com.207
    Resource name:    wordpress-656c879574-b4b5k
    IP/DNS name:      10.1.3.8
    Owner:            System Administrator (Type: Role)
Customer ID:        ABC0751
Enabled features:   DMC
Client Channel status: Online
Client status:      connected
```


## Annotations

The following are the available annotations for credential injection.

| Annotations | Description | Required | Default |
| --- | --- | --- | --- |
| vault.centrify.com/mutate | Indicates whether to perform mutation. This should be set to "yes" or "no" | Yes | "no" |
| vault.centrify.com/tenant-url | Centrify tenant url | Yes | |
| vault.centrify.com/auth-type | Specifies the method for authenticating to Centrify tenant. If "dmc" is used, sidecar-container annotation must be set to "yes". This should be set to "oauth" or "dmc". | Yes | |
| vault.centrify.com/oauth-secret-name | Specifies Kubernetes secret name that is used to store OAuth2 token. This is required if auth-type annotation is set to "oauth". | No | |
| vault.centrify.com/enrollment-code | Enrollment code used by Centrify Client for sidecar injection method. This is required if auth-type annotation is set to "dmc" and sidecar-container annotation is set to "yes" | No | |
| vault.centrify.com/appid | Application ID configured in Centrify Tenant. It must be set if oauth authenticaiton type is used. An OAuth2 Client web application must be configured in Centrify tenant to support oauth2 authentication. | No | |
| vault.centrify.com/scope | OAuth2 scope defined in OAuth2 Client web application or the scope to be created for DMC authentication. For example, it can be set to "aapm" | Yes | |
| vault.centrify.com/init-image | Configures init container image to be used. | No | "centrify/secret-injector-oauth" |
| vault.centrify.com/sidecar-image | Configures sidecar container image to be used. | No | "centrify/secret-injector-dmc" |
| vault.centrify.com/init-container | Specifies whether to inject init container. Unless specifically indicates no, it should always be created to at least copy app launcher binary. This should be set to "yes" or "no" | No | "yes" |
| vault.centrify.com/sidecar-container | Specifies whether to inject sidecar container. If DMC is desired to be used for authenticating to Centrify tenant, sidecar container must be used. This should be set to "yes" or "no" | No | "no" |
| vault.centrify.com/app-launcher | Full path of application launcher binary. This configures how application is launched in original container. Mutate container command so that it is launched by app launcher that "inserts" secrets into environment variables within the process. | No | |
| vault.centrify.com/vaultsecret_\<secret file name\> | Specifies name of secret file and corresponding account password or secret to be checked out from Centrify tenant. <br><br>Format of its value must be "vault://system\|database\|domain/\<system name\>/\<account name\>" or "vault://secret/\<path name\>/.../\<path name\>/\<secret name\>". <br><br>For example, to checkout password for account "dbadmin" in "MSSQL (Demo Lab)" and store it in /centrify/secret/DB_PASSWORD in application container, annotation name should be vault.centrify.com/vaultsecret_DB_PASSWORD with value "vault://database/MSSQL (Demo Lab)/dbadmin". Multiple such annotations can be defined to checkout multiple passwords or secrets. | Yes | |