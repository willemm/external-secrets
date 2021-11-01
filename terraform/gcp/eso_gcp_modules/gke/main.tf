module "gke" {
  source                     = "terraform-google-modules/kubernetes-engine/google"
  version                    = "v17.1.0"
  project_id                 = "${var.project_id}"
  name                       = "${var.env}-cluster"
  regional                   = false
  #  zones                      = ["us-central1-a", "us-central1-b", "us-central1-f"]
  zones                      = "${var.zones}"
  network                    = "${var.network}"
  subnetwork                 = "${var.subnetwork}"
  ip_range_pods              = "${var.ip_pod_range}"
  ip_range_services          = "${var.ip_service_range}"
  http_load_balancing        = true
  horizontal_pod_autoscaling = "${var.horizontal_pod_autoscaling}"
	#kubernetes_dashboard       = false
  network_policy             = false
  logging_service            = "logging.googleapis.com/kubernetes"
  monitoring_service         = "monitoring.googleapis.com/kubernetes"
  kubernetes_version         = "latest"

  node_pools = [
    {
      name               = "default-node-pool"
      machine_type       = "n1-standard-2"
      min_count          = "${var.node_min_count}"
      max_count          = "${var.node_max_count}"
      disk_size_gb       = 50
      disk_type          = "pd-standard"
      image_type         = "COS"
      auto_repair        = true
      auto_upgrade       = true
      preemptible        = var.preemptible
      initial_node_count = "${var.initial_node_count}"
    },
  ]

  node_pools_oauth_scopes = {
    all = []

    default-node-pool = [
      "https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/devstorage.read_only",
			"https://www.googleapis.com/auth/service.management.readonly",
			"https://www.googleapis.com/auth/servicecontrol",
			"https://www.googleapis.com/auth/trace.append"
    ]
  }

  node_pools_labels = {
    all = {}

    default-node-pool = {
      default-node-pool = "true"
    }
  }

  node_pools_metadata = {
    all = {}

    default-node-pool = {
      node-pool-metadata-custom-value = "my-node-pool"
    }
  }

  #  node_pools_taints = {
  #    all = []
  #
  #    default-node-pool = [
  #      {
  #        key    = "default-node-pool"
  #        value  = "true"
  #        effect = "PREFER_NO_SCHEDULE"
  #      },
  #    ]
  #  }

  node_pools_tags = {
    all = []

    default-node-pool = [
      "default-node-pool"#, "allow-http", "allow-https"
    ]
  }
}