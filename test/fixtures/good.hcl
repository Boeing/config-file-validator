default_address = "127.0.0.1"
default_message = upper("Incident: ${incident}")
default_options = {
  priority: "High",
  color: "Red"
}

incident_rules {
    # Rule number 1
    rule "down_server" "infrastructure" {
        incident = 100
        options  = var.override_options ? var.override_options : var.default_options
        server   = default_address
        message  = default_message
    }
}
