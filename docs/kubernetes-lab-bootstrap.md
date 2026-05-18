# Kubernetes lab bootstrap notes

This project assumes a working Kubernetes cluster with:

- cluster-admin access for the current operator
- a CNI plugin
- the standard CNI plugin binaries on each node
- `kube-proxy` installed

The local lab cluster used for this repository was repaired with the following sequence.

## Restore kubeadm admin permissions

If `kubectl` reaches the API server but the `kubernetes-admin` user cannot list cluster
resources, restore the kubeadm admin group binding with the break-glass kubeconfig:

```bash
sudo KUBECONFIG=/etc/kubernetes/super-admin.conf \
kubectl create clusterrolebinding kubeadm-cluster-admins-repair \
  --clusterrole=cluster-admin \
  --group=kubeadm:cluster-admins
```

## Install Flannel

This lab uses the default Flannel-compatible pod network:

```bash
kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
```

## Install standard CNI binaries

Flannel installs its own plugin, but workload pods also need the standard CNI plugins
such as `loopback` and `portmap`:

```bash
curl -L -o /tmp/cni-plugins-linux-amd64-v1.8.0.tgz \
  https://github.com/containernetworking/plugins/releases/download/v1.8.0/cni-plugins-linux-amd64-v1.8.0.tgz
echo 'ab3bda535f9d90766cccc90d3dddb5482003dd744d7f22bcf98186bf8eea8be6  /tmp/cni-plugins-linux-amd64-v1.8.0.tgz' \
  | sha256sum -c -
sudo mkdir -p /opt/cni/bin
sudo tar -C /opt/cni/bin -xzf /tmp/cni-plugins-linux-amd64-v1.8.0.tgz
```

## Restore kube-proxy if missing

If cluster services such as `10.96.0.1:443` do not work and the `kube-proxy`
DaemonSet is absent, recreate the addon:

```bash
sudo kubeadm init phase addon kube-proxy \
  --kubeconfig /etc/kubernetes/admin.conf \
  --pod-network-cidr 10.244.0.0/16
```

## Verify the cluster

```bash
kubectl get nodes -o wide
kubectl get ds -A
kubectl get pods -A
```

Expected healthy shape:

- the node is `Ready`
- `kube-flannel-ds` is available
- `kube-proxy` is available
