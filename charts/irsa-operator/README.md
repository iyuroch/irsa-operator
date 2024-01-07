# irsa-operator

A Helm chart for irsa-operator, an open-source role mapped service account for Kubernetes.

## Installing the Chart

```bash
helm upgrade --install --namespace irsa-operator --create-namespace \
  irsa-operator <link> \
  --version <version> \
  --set "serviceAccount.annotations.eks\.amazonaws\.com/role-arn=${IRSA_OPERATOR_IAM_ROLE_ARN}" \
  --wait
```

## Values
