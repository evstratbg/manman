from litestar.exceptions import HTTPException


class NoTemplateFound(HTTPException):
    status_code = 404
    detail = "No jinja template found for"
