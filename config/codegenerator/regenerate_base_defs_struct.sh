#!/usr/bin/env sh

python3 -m venv venv
. venv/bin/activate

python -m pip install -r requirements.txt

python base_defs_parser_generator.py

cd ..
go fmt
