import os
import sys
sys.path.insert(0, os.path.abspath('../src'))

project = 'manman'
author = 'Bogdan Evstratenko'
release = '0.1.0'

extensions = [
    'myst_parser',
]

templates_path = ['_templates']
exclude_patterns = []

html_theme = 'alabaster'
html_static_path = ['_static']
