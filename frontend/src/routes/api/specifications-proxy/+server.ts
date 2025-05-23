import type { RequestHandler } from './$types';
import { EventSource } from 'eventsource';

type Client = (data: string) => void;

const clients = new Set<Client>();

let upstream: EventSource | null = null;

function startUpstreamStream() {
	if (upstream) return;

	upstream = new EventSource('http://localhost:7070/specifications');

	upstream.onmessage = (event) => {
		console.log('[Proxy] Forwarding:', event.data);
		for (const send of clients) {
			try {
				send(event.data);
			} catch {
				// drop
			}
		}
	};

	upstream.onerror = (err) => {
		console.error('Upstream SSE error:', err);
		upstream?.close();
		upstream = null;
		setTimeout(startUpstreamStream, 5000);
	};
}

startUpstreamStream();

export const GET: RequestHandler = ({ setHeaders }) => {
	setHeaders({
		'Content-Type': 'text/event-stream',
		'Cache-Control': 'no-cache',
		Connection: 'keep-alive'
	});

	const encoder = new TextEncoder();

	const stream = new ReadableStream({
		start(controller) {
			const send = (json: string) => {
				controller.enqueue(encoder.encode(`data: ${json}\n\n`));
			};

			clients.add(send);

			controller.enqueue(encoder.encode(`: connected\n\n`)); // optional comment for handshake

			return () => {
				clients.delete(send);
			};
		}
	});

	return new Response(stream);
};
