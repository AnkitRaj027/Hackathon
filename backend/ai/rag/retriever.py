from pathlib import Path


class KnowledgeRetriever:
    """Replaceable keyword retriever for the small local Vault knowledge base."""

    def __init__(self, knowledge_dir: Path | None = None) -> None:
        self.knowledge_dir = knowledge_dir or Path(__file__).parent / "knowledge"

    def retrieve(self, query: str, limit: int = 3) -> list[dict[str, str]]:
        terms = {term.lower().strip(".,?!:;()") for term in query.split() if len(term) > 2}
        matches: list[tuple[int, str, str]] = []
        for path in sorted(self.knowledge_dir.glob("*.md")):
            content = path.read_text(encoding="utf-8")
            score = sum(content.lower().count(term) for term in terms)
            if score:
                matches.append((score, path.name, content[:4000]))
        matches.sort(key=lambda match: (-match[0], match[1]))
        return [{"source": name, "content": content} for _, name, content in matches[:limit]]