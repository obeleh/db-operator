

# creating a new crd

```
operator-sdk create api --group db-operator --version v1alpha1 --kind <kind-here> --resource --controller
```


# Unit testing

First install [krew](https://krew.sigs.k8s.io/docs/user-guide/setup/install/)

```bash
kubectl krew install kuttl 
kubectl krew install assert   
```

```bash
make kuttl-tests
```
