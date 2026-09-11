"""Local OpenAI-compatible Embeddings API backed by the existing BGE-M3 cache."""
from __future__ import annotations

import asyncio
import logging
import math
import os
import re
import shutil
import subprocess
import sys
import tempfile
import threading
import zipfile
from pathlib import Path
from typing import Any, List, Union
from xml.etree import ElementTree

from fastapi import FastAPI, File, HTTPException, UploadFile
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
try:
    import pymupdf
except ImportError:
    pymupdf = None
try:
    from sentence_transformers import SentenceTransformer
except ImportError:
    SentenceTransformer = None

MODEL = os.getenv("EMBEDDING_MODEL", "BAAI/bge-m3")
CACHE = Path(os.getenv(
    "EMBEDDING_CACHE",
    "/Users/a011/Documents/ChatGPT/智能助手/relay-assistant-mvp/.model-cache",
))

app = FastAPI(title="Local BGE-M3 Embeddings")
app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:3001", "http://127.0.0.1:3001"],
    allow_methods=["*"],
    allow_headers=["*"],
)
_model = None
logger = logging.getLogger("agentdesk.parser")
MINERU = Path(os.getenv(
    "MINERU_BIN",
    "/Users/a011/Documents/ChatGPT/智能助手/relay-assistant-mvp/.mineru-venv/bin/magic-pdf",
))
MINERU_CONFIG = Path(os.getenv(
    "MINERU_CONFIG",
    Path(__file__).resolve().parent / "config" / "mineru.json",
))
EMBEDDING_BATCH_SIZE = max(1, int(os.getenv("EMBEDDING_BATCH_SIZE", "32")))
RELAY_ROOT = Path(os.getenv(
    "RELAY_ASSISTANT_ROOT",
    "/Users/a011/Documents/ChatGPT/智能助手/relay-assistant-mvp",
))
_crawler_lock = threading.Lock()


def summarize_mineru_error(message: str) -> str:
    missing_module = re.search(r"ModuleNotFoundError: No module named ['\"]([^'\"]+)['\"]", message)
    if missing_module:
        return f"MinerU 运行环境缺少依赖：{missing_module.group(1)}"
    lowered = message.lower()
    if "password" in lowered or "encrypted" in lowered:
        return "MinerU 无法解析加密文档，请先移除文档密码"
    return "MinerU 解析失败，请确认文件内容可读取后重试"


def parse_docx_text(path: Path) -> str:
    with zipfile.ZipFile(path) as archive:
        xml = archive.read("word/document.xml")
    root = ElementTree.fromstring(xml)
    namespace = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
    paragraphs = []
    for paragraph in root.iter(namespace + "p"):
        text = "".join(node.text or "" for node in paragraph.iter(namespace + "t")).strip()
        if text:
            paragraphs.append(text)
    return "\n\n".join(paragraphs)


def detect_pdf_method(path: Path, sample_limit: int = 8) -> str:
    """Choose MinerU text mode only when sampled pages have a reliable text layer."""
    if pymupdf is None:
        return "auto"
    try:
        with pymupdf.open(path) as document:
            page_count = document.page_count
            if page_count <= 0:
                return "auto"
            sample_count = min(page_count, sample_limit)
            if sample_count == 1:
                page_indices = [0]
            else:
                page_indices = sorted({
                    round(index * (page_count - 1) / (sample_count - 1))
                    for index in range(sample_count)
                })
            character_counts = []
            for page_index in page_indices:
                text = document.load_page(page_index).get_text("text")
                character_counts.append(len(re.sub(r"\s+", "", text)))
    except Exception:
        logger.exception("Could not inspect PDF text layer for %s", path.name)
        return "auto"

    text_pages = sum(count >= 40 for count in character_counts)
    required_text_pages = max(1, math.ceil(len(character_counts) * 0.75))
    required_characters = max(80, len(character_counts) * 40)
    total_characters = sum(character_counts)
    if text_pages >= required_text_pages and total_characters >= required_characters:
        return "txt"
    if text_pages == 0 or total_characters < 40:
        return "ocr"
    return "auto"


def extract_pdf_text(path: Path) -> str | None:
    """Extract a reliable PDF text layer without invoking MinerU."""
    if pymupdf is None:
        return None
    try:
        with pymupdf.open(path) as document:
            if document.page_count <= 0:
                return None
            pages: list[str] = []
            meaningful_pages = 0
            total_characters = 0
            for page_number, page in enumerate(document, start=1):
                text = page.get_text("text", sort=True).strip()
                character_count = len(re.sub(r"\s+", "", text))
                total_characters += character_count
                if character_count >= 40:
                    meaningful_pages += 1
                if text:
                    pages.append(f"## Page {page_number}\n\n{text}")
            required_pages = max(1, math.ceil(document.page_count * 0.7))
            if meaningful_pages < required_pages or total_characters < document.page_count * 40:
                return None
            return "\n\n".join(pages).strip() or None
    except Exception:
        logger.exception("PyMuPDF extraction failed for %s", path.name)
        return None


def parser_label(method: str) -> str:
    return {
        "txt": "mineru-txt",
        "ocr": "mineru-ocr",
        "auto": "mineru-auto",
    }.get(method, "mineru-auto")


class EmbeddingRequest(BaseModel):
    input: Union[str, List[str]]
    model: Union[str, None] = None


class WebsiteCrawlRequest(BaseModel):
    url: str
    max_pages: int = 30
    max_depth: int = 2


def model():
    global _model
    if _model is None:
        if SentenceTransformer is None:
            raise RuntimeError("sentence-transformers is required for BGE-M3")
        _model = SentenceTransformer(MODEL, cache_folder=str(CACHE))
    return _model


@app.get("/health")
def health() -> dict[str, Any]:
    return {"status": "ok", "model": MODEL, "cache": str(CACHE)}


@app.post("/v1/embeddings")
def embeddings(request: EmbeddingRequest) -> dict[str, Any]:
    texts = [request.input] if isinstance(request.input, str) else request.input
    if not texts or any(not isinstance(text, str) or not text.strip() for text in texts):
        raise HTTPException(status_code=400, detail="input must contain non-empty text")
    try:
        vectors = model().encode(
            texts,
            normalize_embeddings=True,
            batch_size=EMBEDDING_BATCH_SIZE,
            show_progress_bar=False,
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"embedding failed: {exc}") from exc
    data = [{"object": "embedding", "index": i, "embedding": vector.tolist()}
            for i, vector in enumerate(vectors)]
    return {"object": "list", "data": data, "model": request.model or MODEL,
            "usage": {"prompt_tokens": 0, "total_tokens": 0}}


@app.post("/v1/crawl-website")
async def crawl_website_endpoint(payload: WebsiteCrawlRequest) -> dict[str, Any]:
    def run() -> dict[str, Any]:
        if str(RELAY_ROOT) not in sys.path:
            sys.path.insert(0, str(RELAY_ROOT))
        from backend.app import knowledge

        with _crawler_lock:
            previous_pages = knowledge.WEBSITE_MAX_PAGES
            previous_depth = knowledge.WEBSITE_MAX_DEPTH
            try:
                knowledge.WEBSITE_MAX_PAGES = max(1, min(payload.max_pages, 200))
                knowledge.WEBSITE_MAX_DEPTH = max(1, min(payload.max_depth, 5))
                return knowledge.crawl_website(payload.url)
            finally:
                knowledge.WEBSITE_MAX_PAGES = previous_pages
                knowledge.WEBSITE_MAX_DEPTH = previous_depth

    try:
        result = await asyncio.to_thread(run)
    except Exception as exc:
        logger.exception("website crawl failed", extra={"url": payload.url})
        raise HTTPException(status_code=502, detail=f"website crawl failed: {exc}") from exc
    if not result.get("pages"):
        raise HTTPException(status_code=422, detail="website contains no readable pages")
    return result


@app.post("/v1/parse-document")
async def parse_document(file: UploadFile = File(...)) -> dict[str, Any]:
    filename = Path(file.filename or "document").name
    extension = Path(filename).suffix.lower()
    if extension not in {".pdf", ".doc", ".docx", ".ppt", ".pptx", ".png", ".jpg", ".jpeg"}:
        raise HTTPException(status_code=400, detail="unsupported document type")
    with tempfile.TemporaryDirectory(prefix="agentdesk-mineru-") as temp_dir:
        temp_path = Path(temp_dir)
        input_path = temp_path / filename
        output_path = temp_path / "output"
        with input_path.open("wb") as target:
            shutil.copyfileobj(file.file, target)
        if extension == ".docx":
            try:
                content = parse_docx_text(input_path).strip()
            except (KeyError, zipfile.BadZipFile, ElementTree.ParseError) as exc:
                raise HTTPException(status_code=422, detail=f"DOCX 解析失败: {exc}") from exc
            if content:
                return {"filename": filename, "contentType": "markdown", "content": content, "parser": "docx"}
        method = detect_pdf_method(input_path) if extension == ".pdf" else "auto"
        if extension == ".pdf" and method == "txt":
            content = extract_pdf_text(input_path)
            if content:
                logger.info("Parsed %s directly with PyMuPDF", filename)
                return {
                    "filename": filename,
                    "contentType": "markdown",
                    "content": content,
                    "parser": "pymupdf",
                }
            method = "auto"
        if not MINERU.is_file():
            raise HTTPException(status_code=503, detail="MinerU executable is unavailable")
        if not MINERU_CONFIG.is_file():
            raise HTTPException(status_code=503, detail="MinerU 配置文件不可用")
        logger.info("Selected MinerU %s mode for %s", method, filename)
        try:
            process_env = os.environ.copy()
            office_bin = "/Users/a011/.cache/codex-runtimes/codex-primary-runtime/dependencies/bin/override"
            process_env["PATH"] = office_bin + os.pathsep + process_env.get("PATH", "")
            process_env["MINERU_TOOLS_CONFIG_JSON"] = str(MINERU_CONFIG)
            completed = await asyncio.to_thread(
                subprocess.run,
                [str(MINERU), "--path", str(input_path), "--output-dir", str(output_path), "--method", method],
                capture_output=True,
                text=True,
                timeout=900,
                env=process_env,
            )
        except subprocess.TimeoutExpired as exc:
            logger.exception("MinerU timed out while parsing %s", filename)
            raise HTTPException(status_code=504, detail="MinerU parsing timed out") from exc
        if completed.returncode != 0:
            message = (completed.stderr or completed.stdout or "MinerU parsing failed").strip()
            logger.error(
                "MinerU failed for %s with exit code %s\nstdout:\n%s\nstderr:\n%s",
                filename,
                completed.returncode,
                completed.stdout,
                completed.stderr,
            )
            raise HTTPException(status_code=500, detail=summarize_mineru_error(message))
        markdown_files = sorted(output_path.rglob("*.md"), key=lambda path: path.stat().st_size, reverse=True)
        if not markdown_files:
            logger.error(
                "MinerU generated no Markdown for %s\nstdout:\n%s\nstderr:\n%s",
                filename,
                completed.stdout,
                completed.stderr,
            )
            raise HTTPException(status_code=422, detail="MinerU 未生成可读内容，请确认文件内容后重试")
        content = markdown_files[0].read_text(encoding="utf-8", errors="replace").strip()
        if not content:
            raise HTTPException(status_code=422, detail="parsed document is empty")
        return {
            "filename": filename,
            "contentType": "markdown",
            "content": content,
            "parser": parser_label(method),
        }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=int(os.getenv("EMBEDDING_PORT", "8090")))
