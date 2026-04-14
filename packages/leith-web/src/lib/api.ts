export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export interface Post {
    id: string;
    body: string;
    mediaHashes?: string[];
    parentId?: string;
    timestamp: number;
    authorDid: string;
    signature: string;
    visibilityScore?: number;
}

export interface CreatePostRequest {
    body: string;
    mediaHashes: string[];
    parentId?: string;
    timestamp: number;
    authorDid: string;
    signature: string;
}

export async function fetchPosts(limit = 20, offset = 0): Promise<Post[]> {
    const res = await fetch(`${API_BASE_URL}/posts?limit=${limit}&offset=${offset}`, {
        cache: 'no-store',
    });
    if (!res.ok) {
        throw new Error('Failed to fetch posts');
    }
    return res.json();
}

export async function createPost(req: CreatePostRequest): Promise<Post> {
    const res = await fetch(`${API_BASE_URL}/posts`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(req),
    });
    if (!res.ok) {
        throw new Error('Failed to create post');
    }
    return res.json();
}
