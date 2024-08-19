from litestar.exceptions import HTTPException


class NoTemplateFound(HTTPException):
    status_code = 404
    detail = "No jinja template found for"


class ImproperConfig(HTTPException):
    status_code = 400
    detail = "Improper configuration"
