variable "name" {
  default="hello"
    description = "The name"
}

resource "aws_instance" "web" {
ami           = "abc-123"
  instance_type = "t2.micro"
tags = {
Name = "web"
    Environment="prod"
}
}
