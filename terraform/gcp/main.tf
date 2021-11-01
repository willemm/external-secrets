terraform {
  backend "gcs" {
    bucket = "eso-infra-state"
    prefix = "eso-infra-state/state"
    credentials = "secrets/gcloud-service-account-key.json"
  }
}

module "test-network" {
  source = "./eso_gcp_modules/network"
  env = "${var.env}"
  region = "${var.region}"
  ip_cidr_range = "${var.ip_cidr_range}"
}

module "test-cluster" {
  source = "./eso_gcp_modules/gke"
  project_id = "${var.project_id}"
  env = "${var.env}"
  region = "${var.region}"
  zones = ["${var.zone}"]
  network = "${module.test-network.vpc-name}"
  subnetwork = "${module.test-network.subnet-name}"
  ip_pod_range = "${var.env}-pod-ip-range"
  ip_service_range = "${var.env}-service-ip-range"
  horizontal_pod_autoscaling = "${var.horizontal_pod_autoscaling}"
  node_min_count = "${var.node_min_count}"
  node_max_count = "${var.node_max_count}"
  initial_node_count = "${var.initial_node_count}"
  preemptible = true
}

module "my-app-workload-identity" {
  source     = "terraform-google-modules/kubernetes-engine/google//modules/workload-identity"
  use_existing_k8s_sa = true
  cluster_name = "${var.env}-cluster"
  location = "${var.zone}"
  annotate_k8s_sa = false
  name       = "external-secrets"
  namespace  = "eso"
  project_id = "external-secrets-operator"
  roles      = ["roles/secretmanager.secretAccessor"]
}
