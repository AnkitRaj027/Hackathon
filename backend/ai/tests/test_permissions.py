from backend.ai.permissions import RiskLevel, requires_approval


def test_read_operations_do_not_require_approval() -> None:
    assert not requires_approval(RiskLevel.READ)


def test_modifying_and_destructive_operations_require_approval() -> None:
    assert requires_approval(RiskLevel.MODIFY)
    assert requires_approval(RiskLevel.DESTRUCTIVE)