terraform {
  required_providers {
    cloudflare = {
      source = "cloudflare/cloudflare"
      version = "~> 5.7"
    }
  }
}

variable "cloudflare_api_token" {
  description = "Cloudflare API Token with permissions to manage R2 buckets"
  type        = string
  sensitive   = true
}

variable "qovery_cli_releases_cloudflare_r2_bucket_name" {
  description = "Name of the R2 bucket for Qovery CLI releases"
  type        = string
  default     = "qovery-cli-releases"
}

variable "cloudflare_account_id" {
  description = "Cloudflare Account ID where the R2 bucket will be created"
  type        = string
}

variable "qovery_cli_releases_public_domain_name" {
  description = "Public domain name for the Qovery CLI releases bucket"
  type        = string
}

variable "cloudflare_zone_id" {
  description = "Cloudflare Zone ID for the domain"
  type        = string
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}

resource "cloudflare_r2_bucket" "qovery-cli-releases-cloudflare-r2-bucket" {
  account_id = var.cloudflare_account_id
  name       = var.qovery_cli_releases_cloudflare_r2_bucket_name
  location   = "WEUR"
}

resource "cloudflare_r2_custom_domain" "qovery-cli-releases-cloudflare-r2-bucket-custom-domain" {
  depends_on = ["qovery-cli-releases-cloudflare-r2-bucket"]
  account_id = var.cloudflare_account_id
  bucket_name = var.qovery_cli_releases_cloudflare_r2_bucket_name
  domain = var.qovery_cli_releases_public_domain_name
  enabled = true
  zone_id = var.cloudflare_zone_id
}
