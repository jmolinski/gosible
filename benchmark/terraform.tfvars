public_keys = [
    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDBpC+Jwzbh5P15fa+SMMGr4vd6FS5SWZvNttLDiiXKwG7zP1YwudIA0anC75xxMKGC9wsvimcyEABxe+8e02esX1rof20QqmVwvhzVvlUPtb8nSw/AxjInglGmdkMU93aPDGVWbl44RfXk2OaVVWmcWWYMmp0W9YYcunW5Cwxuozd6kfFFrnS5gxwstDNS2ARoaSp1bkKhTKIvXqDMKsIfQTzpGew/G99sV/JdiIhuoIak20d4dxwyJzVJu08sn4MNw7d96kgtvPj/bRdPSM9ofclUmZnMrwUXMBz6Cixspdy5kUpoytmuRguXQxGRQ1bis4HLC/u8IdGaYqRclE7Lk1V4rq66cSj0TR+TSKAlHXCqf81v8QX5JFc4IeaBEBxMyTlBnDfA3Y9ssM15SNuQ9by74kl93ReUcIs0aHSR/glSZaFpFnwhhT/3NQgV5UKpy72TPXYptBqBHC0Em18L05CxwHBEiEBKTULSFNwFy8P5n01kAjceFSXrGbNtlBM= lynczu@Rafas-MacBook-Pro.local",
    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDXCZCnd3k4ohBy3C8ruYo4kWlM72oFBtiMKaeWaSoU5 mcmrarm@gmail.com",
    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCv4gQd2qY+gGJHA3HWy1AvBRpNmNy/00lgTg9T0QcQMCHFID85GmkDbrXekSh5iHKCdRh1pzoPKyI5bSBSlyQYK9bJbP+TOPfqeaZzAdPcChy6XCXXUzCiXFLSxv7QZaPqw3JG4gbpOutsHwp4d3mTJVY5tJGGV5BHwzboZNSu4AtHRmNSMnIh0rWKH/Ur6iDNKvtA8ZxqtoNYgiCSfowBVDk6/1o6jvUjV54Gzbiy3Gu8Mn3ipHVgEYlLKISHEiSoLaXLn9mwRlFN01Vb6Wpwce6/BvI5yBOj6BIUyQkYUtQV+WQVyBXu14n3O9grUlZI4t05Rl56911ZDeyb50Lh7gIub5TaB5CNGs/5WJWgOJA8dI/CemoeozXO9dr278o5RgYlj775/iqNuV/MkXPDJJVI3hWFSVpBF8K6I3JOBIiCHZo0betskzuynV2muAWJxzan7GPMn3+oJyTVSEZV+R7nztd1Z59444pzEQSb2CUlylfE7nst5HoDq4//IN8= jakubmolinski@jakubs-mbp.home",
    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC65PxZEw5vu9IuLdFW8htzz7Duw7dYFdy+oEC/Cm5B3LKWf6DExyJ2KhbSf2uteEIFhMzWAQzlwvt8UlDC4Uzy4Ly/g1xVlcuoVpYOL37L/FaNbeqkOa3qQT795nJRVKfAjaNUuHr60PmWR7vO/J29B25Y4BR75lQsMA8RXzoSLo2iz3ffjDEe8L26RT3Cx7v5CIH1MAjnJPozdcoKY9Eq9jM33oZTHjDmE8vpswHZB8N6e62cdSbNoj/ck5RQtQQ2q0Qll8TqCn3grI91np/24rvCSJHrnxEnDJTIhNUvgsVAMOCmXyv6zZHixvbGTyFgXhsKAbshL80RHJ1oXhMnaZfkeAK0OXihP/5LsyLSCI4dPmXwAu/VQ+LpgrQ+r3ZDx4CU/Jbmbk6kOdbgdfpIQq0qtxAm/x/rolyFMTzFxYaX25/DmGPDoztqIGDipT08ThYkpi//44UEw1dBuCnmj3ewn/mR17J/5uO3nf6dCdQS0dsfPK5mkWoFl2libhk= hb417666@students"
>>>>>>> 25ef9cc (deploy changes (#186))
]

ingress = [
    {
        cidr = "89.64.77.20/32"
        port = 22
    },
    {
        cidr = "193.0.96.129/32" # students
        port = 22
    },
    {
        cidr = "10.0.0.0/16"
        port = 22
    },
    {
        cidr = "10.0.0.0/16"
        port = 2222
    },
]

tags = {
    Owner       = "rjeczalik"
    Project     = "gosible"
    Environment = "lab"
    keep        = "48"
}
