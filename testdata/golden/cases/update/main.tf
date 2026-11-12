resource "terraform_data" "changed" {
  input = "after value"
  count = 3
}
