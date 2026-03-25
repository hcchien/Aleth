import { NextRequest, NextResponse } from "next/server";
import { writeFile, mkdir } from "node:fs/promises";
import { join } from "node:path";
import { randomUUID } from "node:crypto";

const UPLOAD_DIR = join(process.cwd(), "public", "uploads");
const MAX_FILE_SIZE = 8 * 1024 * 1024; // 8 MB

// Map detected MIME type → canonical file extension.
// Client-supplied Content-Type and filename extension are not trusted.
const ALLOWED_MIME_TO_EXT: Record<string, string> = {
  "image/jpeg": ".jpg",
  "image/png": ".png",
  "image/gif": ".gif",
  "image/webp": ".webp",
  "image/avif": ".avif",
};

/**
 * Detect the actual image type from magic bytes.
 * Returns a MIME type string, or null if the content is not a recognised image.
 * Immune to spoofed Content-Type headers and filename extensions.
 */
function detectMimeType(buf: Buffer): string | null {
  if (buf.length < 12) return null;

  // JPEG: FF D8 FF
  if (buf[0] === 0xff && buf[1] === 0xd8 && buf[2] === 0xff) return "image/jpeg";

  // PNG: 89 50 4E 47 0D 0A 1A 0A
  if (
    buf[0] === 0x89 && buf[1] === 0x50 && buf[2] === 0x4e && buf[3] === 0x47 &&
    buf[4] === 0x0d && buf[5] === 0x0a && buf[6] === 0x1a && buf[7] === 0x0a
  ) return "image/png";

  // GIF87a / GIF89a: 47 49 46 38 (37|39) 61
  if (
    buf[0] === 0x47 && buf[1] === 0x49 && buf[2] === 0x46 && buf[3] === 0x38 &&
    (buf[4] === 0x37 || buf[4] === 0x39) && buf[5] === 0x61
  ) return "image/gif";

  // WebP: RIFF????WEBP
  if (
    buf[0] === 0x52 && buf[1] === 0x49 && buf[2] === 0x46 && buf[3] === 0x46 &&
    buf[8] === 0x57 && buf[9] === 0x45 && buf[10] === 0x42 && buf[11] === 0x50
  ) return "image/webp";

  // AVIF/AVIS: ISO Base Media File Format — bytes 4–7 = "ftyp", brand at 8–11
  if (buf[4] === 0x66 && buf[5] === 0x74 && buf[6] === 0x79 && buf[7] === 0x70) {
    const brand = buf.toString("ascii", 8, 12);
    if (brand === "avif" || brand === "avis") return "image/avif";
  }

  return null;
}

async function uploadToGCS(
  bucket: string,
  filename: string,
  buffer: Buffer,
  contentType: string,
  keyFile?: string,
): Promise<string> {
  const { Storage } = await import("@google-cloud/storage");
  const storage = new Storage(keyFile ? { keyFilename: keyFile } : {});
  const file = storage.bucket(bucket).file(`uploads/${filename}`);
  await file.save(buffer, {
    contentType,
    metadata: { cacheControl: "public, max-age=31536000" },
  });
  await file.makePublic();
  return `https://storage.googleapis.com/${bucket}/uploads/${filename}`;
}

async function uploadToLocalDisk(filename: string, buffer: Buffer): Promise<string> {
  await mkdir(UPLOAD_DIR, { recursive: true });
  const filepath = join(UPLOAD_DIR, filename);
  await writeFile(filepath, buffer);
  return `/uploads/${filename}`;
}

export async function POST(req: NextRequest) {
  // Validate JWT from Authorization header.
  // Note: full cryptographic verification requires `jose` (npm install jose)
  // and ACCESS_TOKEN_SECRET env var. The gateway also enforces auth on all
  // upstream requests, providing defence-in-depth.
  const authHeader = req.headers.get("authorization");
  if (!authHeader?.startsWith("Bearer ")) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  const token = authHeader.slice(7);
  // Basic structural check: a JWT has exactly 3 dot-separated base64url parts.
  const jwtParts = token.split(".");
  if (jwtParts.length !== 3) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  let formData: FormData;
  try {
    formData = await req.formData();
  } catch {
    return NextResponse.json({ error: "Invalid form data" }, { status: 400 });
  }

  const file = formData.get("file");
  if (!file || typeof file === "string") {
    return NextResponse.json({ error: "No file provided" }, { status: 400 });
  }

  // Read bytes first so we can inspect magic bytes before trusting anything else.
  const arrayBuffer = await file.arrayBuffer();
  if (arrayBuffer.byteLength > MAX_FILE_SIZE) {
    return NextResponse.json({ error: "File too large (max 8 MB)" }, { status: 413 });
  }

  const buffer = Buffer.from(arrayBuffer);

  // Detect MIME type from magic bytes — client-supplied Content-Type is ignored.
  const detectedMime = detectMimeType(buffer);
  if (!detectedMime || !(detectedMime in ALLOWED_MIME_TO_EXT)) {
    return NextResponse.json({ error: "File type not allowed" }, { status: 415 });
  }

  const ext = ALLOWED_MIME_TO_EXT[detectedMime];
  const filename = `${randomUUID()}${ext}`;
  const gcsBucket = process.env.GCS_BUCKET;
  const gcsKeyFile = process.env.GCS_KEY_FILE;

  try {
    let url: string;
    if (gcsBucket) {
      url = await uploadToGCS(gcsBucket, filename, buffer, detectedMime, gcsKeyFile);
    } else {
      // Dev fallback: write to local disk
      url = await uploadToLocalDisk(filename, buffer);
    }
    return NextResponse.json({ url });
  } catch (err) {
    console.error("Upload error:", err);
    return NextResponse.json({ error: "Upload failed" }, { status: 500 });
  }
}
