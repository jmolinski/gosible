provider "aws" {
    region = var.region
}

locals {
    tags = merge(var.tags, tomap({ Deployment = var.deployment }))
}

resource "tls_private_key" "default" {
    algorithm = "RSA"
    rsa_bits = "2048"
}

data "aws_availability_zones" "available" {
    state = "available"
}

resource "aws_instance" "default" {
    ami               = var.instance.ami
    instance_type     = var.instance.type
    key_name          = aws_key_pair.default.key_name
    monitoring        = true
    availability_zone = data.aws_availability_zones.available.names[0]
    subnet_id         = aws_subnet.default.id

    user_data_replace_on_change = true

    user_data = <<-EOF
    #!/bin/bash

    set -euo pipefail

    useradd -m -p "kawLFTG.Brw3o" ${var.user}

    usermod -aG sudo ${var.user}

    mkdir -p /home/${var.user}/.ssh
    touch /home/${var.user}/.ssh/authorized_keys
    chmod 0700 /home/${var.user}/.ssh
    chmod 0600 /home/${var.user}/.ssh/authorized_keys
    chown -R gosible:gosible /home/${var.user}/.ssh

    cat >> /home/${var.user}/.ssh/authorized_keys <<EOL
    ${join("\n", var.public_keys)}
    EOL

    EOF

    vpc_security_group_ids = [
        aws_security_group.default.id,
    ]

    credit_specification {
        cpu_credits = "unlimited"
    }

    tags  = local.tags
    count = var.instance.count
}

resource "aws_key_pair" "default" {
    key_name   = "gosible-${var.deployment}"
    public_key = tls_private_key.default.public_key_openssh

    tags = local.tags
}

resource "aws_vpc" "default" {
    cidr_block = "10.0.0.0/16"
    tags = local.tags
}

resource "aws_internet_gateway" "default" {
    vpc_id = aws_vpc.default.id
    tags   = local.tags
}

resource "aws_subnet" "default" {
    availability_zone = data.aws_availability_zones.available.names[0]
    cidr_block        = "10.0.1.0/24"
    vpc_id            = aws_vpc.default.id
    map_public_ip_on_launch = true

    tags = local.tags
}

resource "aws_eip" "default" {
    vpc      = true
    instance = element(aws_instance.default.*.id, count.index)

    tags  = local.tags
    count = var.instance.count
}

resource "aws_route_table" "default" {
    vpc_id = aws_vpc.default.id

    route {
        cidr_block = "0.0.0.0/0"
        gateway_id = aws_internet_gateway.default.id
    }

    tags = local.tags
}

resource "aws_route_table_association" "default" {
    route_table_id = aws_route_table.default.id
    subnet_id      = aws_subnet.default.id
}

resource "aws_security_group" "default" {
    name        = "gosible-${var.deployment}"
    vpc_id      = aws_vpc.default.id

    tags = local.tags
}

resource "aws_security_group_rule" "egress" {
    type        = "egress"
    cidr_blocks = ["0.0.0.0/0"]
    from_port   = "0"
    to_port     = "0"
    protocol    = "-1"

    security_group_id = aws_security_group.default.id
}

resource "aws_security_group_rule" "ingress" {
    type        = "ingress"
    cidr_blocks = [element(var.ingress.*.cidr, count.index)]
    from_port   = element(var.ingress.*.port, count.index)
    to_port     = element(var.ingress.*.port, count.index)
    protocol    = "tcp"

    security_group_id = aws_security_group.default.id

    count = length(var.ingress)
}

output "ip" {
    value = aws_eip.default.*.public_ip
}
