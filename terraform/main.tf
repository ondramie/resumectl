terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

provider "google" {}

data "google_billing_account" "default" {
  display_name = var.billing_account_name
}

resource "random_id" "suffix" {
  byte_length = 4
}

resource "google_project" "resumectl" {
  name            = "resumectl"
  project_id      = "resumectl-${random_id.suffix.hex}"
  billing_account = data.google_billing_account.default.id
}

resource "google_project_service" "gmail" {
  project            = google_project.resumectl.project_id
  service            = "gmail.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "iap" {
  project            = google_project.resumectl.project_id
  service            = "iap.googleapis.com"
  disable_on_destroy = false
}

resource "google_iap_brand" "resumectl" {
  support_email     = var.support_email
  application_title = "resumectl"
  project           = google_project.resumectl.number

  depends_on = [google_project_service.iap]
}

resource "google_iap_client" "resumectl" {
  display_name = "resumectl"
  brand        = google_iap_brand.resumectl.name
}
