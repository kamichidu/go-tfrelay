resource "terraform_data" "fixture" {
  input = {
    explicit = "declared value"
    number   = 7
  }
}

resource "terraform_data" "dependent" {
  input = {
    explicit    = "dependent value"
    upstream_id = terraform_data.fixture.id
  }
}

resource "random_password" "defaulted" {
  length = 24
  lower  = false

  keepers = {
    upstream_id = terraform_data.fixture.id
  }
}

resource "time_offset" "fixed" {
  base_rfc3339 = "2030-01-01T00:00:00Z"
  offset_days  = 1
}

output "fixture_id" {
  value = terraform_data.fixture.id
}
