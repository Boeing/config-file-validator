locals {
  name = "app"
  env  = "prod"
}

output "id" {
  value = local.name
}
