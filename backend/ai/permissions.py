from enum import Enum


class RiskLevel(str, Enum):
	READ = "LOW"
	MODIFY = "MEDIUM"
	DESTRUCTIVE = "HIGH"


class ActionStatus(str, Enum):
	PENDING = "PENDING"
	APPROVED = "APPROVED"
	REJECTED = "REJECTED"
	EXECUTED = "EXECUTED"
	EXPIRED = "EXPIRED"
	FAILED = "FAILED"


def requires_approval(risk_level: RiskLevel) -> bool:
	return risk_level in {RiskLevel.MODIFY, RiskLevel.DESTRUCTIVE}


def destructive_warning(tool_name: str) -> str:
	return f"{tool_name} can permanently remove Vault data or capacity. Review the target carefully before approving."
