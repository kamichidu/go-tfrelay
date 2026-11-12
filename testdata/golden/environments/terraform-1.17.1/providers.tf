terraform {
  required_version = "= 1.17.1"

  required_providers {
    random = {
      source  = "hashicorp/random"
      version = "= 3.7.2"
    }
    time = {
      source  = "hashicorp/time"
      version = "= 0.13.1"
    }
  }
}
