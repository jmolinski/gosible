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
