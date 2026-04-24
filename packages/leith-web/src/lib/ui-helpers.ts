export function firstLine(body: string) {
  return body.split("\n")[0].trim() || "Untitled";
}

export function truncate(body: string, max: number) {
  if (body.length <= max) {
    return body;
  }
  return `${body.slice(0, max).trim()}...`;
}

export function formatAuthor(did: string) {
  if (did.startsWith("oauth:")) {
    return "訪客";
  }
  if (did.startsWith("did:vflow:")) {
    return `did:...${did.slice(-6)}`;
  }
  return did;
}

export function formatDate(value: string) {
  const date = new Date(value);
  return date.toLocaleString("zh-TW", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function trustRequirementLabel(level: number) {
  return `需要 L${level} 以上信任等級`;
}
