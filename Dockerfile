FROM mirror.gcr.io/python:3.12

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE 1
ENV PYTHONUNBUFFERED 1

COPY requirements.lock README.md pyproject.toml ./

RUN pip install -U pip setuptools && pip install --no-cache-dir -r requirements.lock

COPY . /app

CMD uvicorn src.app:app --port 8000 --host 0.0.0.0
