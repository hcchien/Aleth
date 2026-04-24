type JsonRecord = Record<string, unknown>;

function base64UrlToBuffer(value: string): ArrayBuffer {
  const padding = "=".repeat((4 - (value.length % 4)) % 4);
  const base64 = (value + padding).replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(base64);
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
  return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
}

function bytesToBase64Url(bytes: Uint8Array): string {
  const binary = Array.from(bytes, (byte) => String.fromCharCode(byte)).join("");
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function toCreationOptions(publicKey: JsonRecord): CredentialCreationOptions {
  const user = publicKey.user as JsonRecord;
  const excludeCredentials = Array.isArray(publicKey.excludeCredentials)
    ? publicKey.excludeCredentials.map((credential) => {
        const descriptor = credential as JsonRecord;
        return {
          ...descriptor,
          id: base64UrlToBuffer(String(descriptor.id)),
        };
      })
    : [];

  return {
    publicKey: {
      ...publicKey,
      challenge: base64UrlToBuffer(String(publicKey.challenge)),
      user: {
        ...user,
        id: base64UrlToBuffer(String(user.id)),
      },
      excludeCredentials: excludeCredentials as PublicKeyCredentialDescriptor[],
    } as PublicKeyCredentialCreationOptions,
  };
}

function toRequestOptions(publicKey: JsonRecord): CredentialRequestOptions {
  const allowCredentials = Array.isArray(publicKey.allowCredentials)
    ? publicKey.allowCredentials.map((credential) => {
        const descriptor = credential as JsonRecord;
        return {
          ...descriptor,
          id: base64UrlToBuffer(String(descriptor.id)),
        };
      })
    : [];

  return {
    publicKey: {
      ...publicKey,
      challenge: base64UrlToBuffer(String(publicKey.challenge)),
      allowCredentials: allowCredentials as PublicKeyCredentialDescriptor[],
    } as PublicKeyCredentialRequestOptions,
  };
}

export function parseCreationOptions(publicKey: JsonRecord): CredentialCreationOptions {
  const parser = PublicKeyCredential as typeof PublicKeyCredential & {
    parseCreationOptionsFromJSON?: (options: JsonRecord) => CredentialCreationOptions;
  };
  if (typeof parser.parseCreationOptionsFromJSON === "function") {
    return parser.parseCreationOptionsFromJSON({ publicKey });
  }
  return toCreationOptions(publicKey);
}

export function parseRequestOptions(publicKey: JsonRecord): CredentialRequestOptions {
  const parser = PublicKeyCredential as typeof PublicKeyCredential & {
    parseRequestOptionsFromJSON?: (options: JsonRecord) => CredentialRequestOptions;
  };
  if (typeof parser.parseRequestOptionsFromJSON === "function") {
    return parser.parseRequestOptionsFromJSON({ publicKey });
  }
  return toRequestOptions(publicKey);
}

export function credentialToJSON(credential: PublicKeyCredential): Record<string, unknown> {
  const response = credential.response as AuthenticatorAssertionResponse | AuthenticatorAttestationResponse;
  const json: Record<string, unknown> = {
    id: credential.id,
    rawId: bytesToBase64Url(new Uint8Array(credential.rawId)),
    type: credential.type,
    clientExtensionResults: credential.getClientExtensionResults(),
  };

  if ("attestationObject" in response) {
    json.response = {
      attestationObject: bytesToBase64Url(new Uint8Array(response.attestationObject)),
      clientDataJSON: bytesToBase64Url(new Uint8Array(response.clientDataJSON)),
      transports: typeof response.getTransports === "function" ? response.getTransports() : [],
    };
    return json;
  }

  json.response = {
    authenticatorData: bytesToBase64Url(new Uint8Array(response.authenticatorData)),
    clientDataJSON: bytesToBase64Url(new Uint8Array(response.clientDataJSON)),
    signature: bytesToBase64Url(new Uint8Array(response.signature)),
    userHandle: response.userHandle ? bytesToBase64Url(new Uint8Array(response.userHandle)) : null,
  };
  return json;
}
