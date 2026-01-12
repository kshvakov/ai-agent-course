"""
Setup script for MkDocs plugins.
"""
from setuptools import setup, find_packages

setup(
    name="mkdocs-ai-agent-course-plugins",
    version="0.1.0",
    packages=find_packages(where=".mkdocs"),
    package_dir={"": ".mkdocs"},
    install_requires=[
        "mkdocs>=1.5.0",
    ],
    entry_points={
        "mkdocs.plugins": [
            "auto_description = mkdocs_plugins.auto_description:AutoDescriptionPlugin",
        ],
    },
)
