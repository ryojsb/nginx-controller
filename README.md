# nginx-controller
It's the controller to create resources that manage the deploymet which have latest nginx image.

## deploy
Before deploy the resources, you need to execute the commands below.

1. Install CRD.

```
$ make install
```

2. Run custom controller on your terminal.

```
$ make run
```

Then you can deploy the resources with following manifest.

```yaml
apiVersion: nginxcontroller.my.domain/v1
kind: Nginx
metadata:
  name: nginx-sample
spec:
  # Add fields here
  deploymentName: test
  replicas: 3
```

After that you can see outputs with `kubectl get`.

```
% kubectl get nx 
NAME           DEPLOYMENT   AVAILABLEREPLICAS   DEPLOYMENTSTATUS
nginx-sample   test         3                   3/3

% kubectl get deploy
NAME   READY   UP-TO-DATE   AVAILABLE   AGE
test   3/3     3            3           3m27s
```
