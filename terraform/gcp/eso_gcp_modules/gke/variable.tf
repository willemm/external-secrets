variable "project_id" {
  default = "my-project-1475718618821"
}
variable "env" {
  default = "dev"
}
variable "region" {
  default = "us-central1"
}
variable "zones" {
  default = ["us-central1-a", "us-central1-b", "us-central1-f"]
}
variable "network" {
  default = "dev-vpc"
}
variable "subnetwork" {
  default = "dev-subnetwork"
}
variable "ip_pod_range" {
  default = "dev-pod-ip-range"
}
variable "ip_service_range" {
  default = "dev-service-ip-range"
}
variable "horizontal_pod_autoscaling" {
  default = false
}
variable "node_min_count" {
  default = 1
}
variable "node_max_count" {
  default = 5
}
variable "initial_node_count" {
  default = 1
}
variable "preemptible" {
  default = true
}
