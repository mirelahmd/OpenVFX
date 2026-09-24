"""Swappable LLM client for the Creative Director.

Deliberately built on the standard library only. No vendor SDK is imported,
because the routing philosophy of this project is that engines are replaceable
capabilities: adding an OpenAI-compatible or Anthropic route later must not
change the creative treatment contract, and must not add a dependency.

Configuration arrives as a plain dict resolved by the Go runtime from
`byom-video.yaml`, so YAML parsing stays in one place.
"""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from typing import Any, Optional


class LLMError(RuntimeError):
    """Raised when a configured model could not produce usable output."""


class LLMClient:
    """Interface every route must satisfy."""

    name: str = "none"
    provider: str = ""
    model: str = ""
    backend: str = ""

    def available(self) -> bool:
        raise NotImplementedError

    def complete(self, system: str, user: str) -> str:
        raise NotImplementedError


class NullClient(LLMClient):
    """The 'no model configured' route.

    It never fabricates output. Calling it is a programming error; the graph is
    expected to check `available()` and take the deterministic path instead.
    """

    name = "none"

    def __init__(self, reason: str = "no model configured") -> None:
        self.reason = reason

    def available(self) -> bool:
        return False

    def complete(self, system: str, user: str) -> str:
        raise LLMError(self.reason)


class OllamaClient(LLMClient):
    """Local Ollama route - the first working LLM backend.

    Ollama is the default because it keeps the whole creative loop local, which
    is the posture the rest of OpenVFX takes.
    """

    name = "ollama"
    provider = "ollama"

    def __init__(
        self,
        model: str,
        base_url: str = "http://localhost:11434",
        timeout_seconds: int = 120,
        temperature: Optional[float] = None,
    ) -> None:
        self.model = model
        self.backend = (base_url or "http://localhost:11434").rstrip("/")
        self.timeout_seconds = timeout_seconds or 120
        self.temperature = temperature

    def available(self) -> bool:
        return bool(self.model)

    def complete(self, system: str, user: str) -> str:
        payload: dict[str, Any] = {
            "model": self.model,
            "prompt": user,
            "system": system,
            "stream": False,
            "format": "json",
        }
        options: dict[str, Any] = {}
        if self.temperature is not None:
            options["temperature"] = self.temperature
        if options:
            payload["options"] = options

        request = urllib.request.Request(
            f"{self.backend}/api/generate",
            data=json.dumps(payload).encode("utf-8"),
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                body = response.read().decode("utf-8")
        except urllib.error.URLError as exc:
            raise LLMError(f"ollama request failed: {exc.reason}") from exc
        except OSError as exc:  # socket timeouts and friends
            raise LLMError(f"ollama request failed: {exc}") from exc

        try:
            parsed = json.loads(body)
        except json.JSONDecodeError as exc:
            raise LLMError(f"ollama returned non-JSON envelope: {exc}") from exc

        text = parsed.get("response", "")
        if not str(text).strip():
            raise LLMError("ollama returned an empty response")
        return str(text)


class OpenAICompatibleClient(LLMClient):
    """Any endpoint speaking the OpenAI chat-completions shape.

    Included so the seam is real rather than theoretical: swapping routes must
    not touch the treatment contract. It is config-driven and carries no SDK.
    """

    name = "openai-compatible"
    provider = "openai-compatible"

    def __init__(
        self,
        model: str,
        base_url: str,
        api_key: str = "",
        timeout_seconds: int = 120,
        temperature: Optional[float] = None,
    ) -> None:
        self.model = model
        self.backend = (base_url or "").rstrip("/")
        self.api_key = api_key
        self.timeout_seconds = timeout_seconds or 120
        self.temperature = temperature

    def available(self) -> bool:
        return bool(self.model and self.backend)

    def complete(self, system: str, user: str) -> str:
        payload: dict[str, Any] = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": system},
                {"role": "user", "content": user},
            ],
            "response_format": {"type": "json_object"},
        }
        if self.temperature is not None:
            payload["temperature"] = self.temperature

        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        request = urllib.request.Request(
            f"{self.backend}/chat/completions",
            data=json.dumps(payload).encode("utf-8"),
            headers=headers,
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                body = response.read().decode("utf-8")
        except urllib.error.URLError as exc:
            raise LLMError(f"request failed: {exc.reason}") from exc
        except OSError as exc:
            raise LLMError(f"request failed: {exc}") from exc

        try:
            parsed = json.loads(body)
            return str(parsed["choices"][0]["message"]["content"])
        except (json.JSONDecodeError, KeyError, IndexError) as exc:
            raise LLMError(f"unexpected response shape: {exc}") from exc


def build_client(config: dict[str, Any]) -> LLMClient:
    """Construct the configured route.

    Returns a NullClient carrying a reason when nothing usable is configured -
    never a client that would invent output.
    """
    if not config:
        return NullClient("no director config supplied")

    mode = str(config.get("mode", "")).strip().lower()
    if mode in ("", "deterministic", "none", "off"):
        return NullClient("reasoning mode is deterministic; no model requested")

    provider = str(config.get("provider", "")).strip().lower() or "ollama"
    model = str(config.get("model", "")).strip()
    base_url = str(config.get("base_url", "")).strip()
    timeout = int(config.get("timeout_seconds") or 120)
    temperature = config.get("temperature")
    if temperature is not None:
        try:
            temperature = float(temperature)
        except (TypeError, ValueError):
            temperature = None
        if temperature is not None and temperature < 0:
            temperature = None

    if not model:
        return NullClient(f"provider {provider!r} selected but no model configured")

    if provider == "ollama":
        return OllamaClient(
            model=model,
            base_url=base_url or "http://localhost:11434",
            timeout_seconds=timeout,
            temperature=temperature,
        )
    if provider in ("openai-compatible", "openai", "custom-http"):
        if not base_url:
            return NullClient(f"provider {provider!r} requires a base_url")
        return OpenAICompatibleClient(
            model=model,
            base_url=base_url,
            api_key=str(config.get("api_key", "")),
            timeout_seconds=timeout,
            temperature=temperature,
        )

    return NullClient(f"unsupported provider {provider!r}")


def extract_json(raw: str) -> dict[str, Any]:
    """Pull a JSON object out of a model response.

    Models wrap JSON in prose and code fences even when told not to, so this
    strips fences and falls back to the outermost brace pair. A malformed
    response raises rather than returning a partial object - the caller must be
    able to tell the difference between "the model said something unusable" and
    "the model said nothing".
    """
    text = (raw or "").strip()
    if not text:
        raise LLMError("empty model response")

    if text.startswith("```"):
        lines = text.splitlines()
        lines = lines[1:]
        if lines and lines[-1].strip().startswith("```"):
            lines = lines[:-1]
        text = "\n".join(lines).strip()

    try:
        parsed = json.loads(text)
    except json.JSONDecodeError:
        start = text.find("{")
        end = text.rfind("}")
        if start == -1 or end == -1 or end <= start:
            raise LLMError("model response contained no JSON object")
        try:
            parsed = json.loads(text[start : end + 1])
        except json.JSONDecodeError as exc:
            raise LLMError(f"model response was not valid JSON: {exc}") from exc

    if not isinstance(parsed, dict):
        raise LLMError(f"model returned {type(parsed).__name__}, expected an object")
    return parsed
