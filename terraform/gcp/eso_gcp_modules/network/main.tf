resource "google_compute_network" "env-vpc" {
  name          =  "${var.env}-vpc"
  auto_create_subnetworks = "false"
}

resource "google_compute_subnetwork" "env-public-subnet" {
  name = "${var.env}-subnetwork"
  private_ip_google_access = true
  ip_cidr_range = "${var.ip_cidr_range}"
  secondary_ip_range {
    range_name    = "${var.env}-pod-ip-range"
    ip_cidr_range = "${var.ip_pod_range}"
  }
  secondary_ip_range {
    range_name    = "${var.env}-service-ip-range"
    ip_cidr_range = "${var.ip_service_range}"
  }
  region = "${var.region}"
  network = "${google_compute_network.env-vpc.self_link}"
}

output "pod-ip-range" {
  value = "${var.ip_pod_range}"
}
output "vpc-name" {
  value = "${google_compute_network.env-vpc.name}"
}
output "vpc-object" {
  value = "${google_compute_network.env-vpc.self_link}"
}
output "subnet-name" {
  value = "${google_compute_subnetwork.env-public-subnet.name}"
}
output "subnet-ip_cidr_range" {
  value = "${google_compute_subnetwork.env-public-subnet.ip_cidr_range}"
}