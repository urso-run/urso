FROM squidfunk/mkdocs-material:latest

# MkDocs Material comes with most essential plugins pre-installed.
# If you need specific extra plugins, you can install them here:
# RUN pip install --no-cache-dir \
#     mkdocs-redirects \
#     mkdocs-minify-plugin

# Set the working directory to /docs (the expected mount point)
WORKDIR /docs

# Ensure the logo is available in the image
# Note: The Makefile ensures the logo is copied to the build context before building.
COPY docs/static/urso-logo.png docs/static/urso-logo.png

# Expose the default port for mkdocs serve
EXPOSE 8000
