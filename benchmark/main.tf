terraform {
    backend "s3" {
        bucket = "terraform-gosible"
        key    = "gosible"
        region = "us-east-1"
    }
}

module "ubuntu-instances" {
    source = "./instance"

    region     = "us-east-1"
    deployment = "ubuntu-2204"
    user       = "gosible"

    instance = {
        count = 32
        ami   = "ami-09d56f8956ab235b3" # Ubuntu 22.04
        type  = "t3a.small"
    }

    public_keys = var.public_keys
    ingress     = var.ingress
    tags        = var.tags
}

output "ubuntu-instances" {
    value = module.ubuntu-instances.ip
}

module "amazon-instances" {
    source = "./instance"

    region     = "eu-central-1"
    deployment = "amazon-2"
    user       = "gosible"

    instance = {
        count = 1
        ami   = "ami-09439f09c55136ecf" # Amazon Linux
        type  = "t3a.small"
    }

    public_keys = var.public_keys
    ingress     = var.ingress
    tags        = var.tags
}

output "amazon-instances" {
    value = module.amazon-instances.ip
}

module "ubuntu-instances-eu-west-2" {
    source = "./instance"

    region     = "eu-west-2"
    deployment = "ubuntu-2204"
    user       = "gosible"

    instance = {
        count = 8
        ami   = "ami-0a244485e2e4ffd03" # Ubuntu 22.04
        type  = "t3.micro"
    }

    public_keys = var.public_keys
    ingress     = var.ingress
    tags        = var.tags
}

output "ubuntu-instances-eu-west-2" {
    value = module.ubuntu-instances-eu-west-2.ip
}
