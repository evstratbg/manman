from litestar.exceptions import HTTPException


class NoTemplateFound(HTTPException):
    status_code = 404
    detail = "No jinja template found for"


class ImproperVaultConfig(HTTPException):
    status_code = 500
    detail = "Need to provide vault configuration for requesting vault secrets"
