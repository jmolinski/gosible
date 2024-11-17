variable "region" {
    type = string
}

variable "deployment" {
    type = string
}

variable "user" {
    type = string
}

variable "instance" {
    type = object({
        count = number
        ami   = string
        type  = string
    })
}

variable "public_keys" {
    type = list(string)
}

variable "ingress" {
    type = list(object({
        cidr = string
        port = number
    }))
}

variable "tags" {
    type = map
}
