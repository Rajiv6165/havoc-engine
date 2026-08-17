# Havoc Dashboard

Havoc Dashboard is the front-end control plane for **Havoc Engine**, a chaos engineering tool for Kubernetes environments.

This dashboard provides a centralized UI to monitor the health of your services, trigger chaos experiments, and view real-time experiment logs via WebSocket connections to the engine backend.

## Features

- **System Status Overview**: Displays the health, status, and pod count for configured Kubernetes services.
- **Chaos Controls**: Trigger various chaos experiments (Kill Pod, Inject Latency, Spike CPU, Memory Leak) with a single click.
- **Live Experiment Logs**: Real-time streaming log panel that connects to the Havoc Engine WebSocket server to display active experiment events and progress.

## Tech Stack

- Next.js 14 (App Router)
- React
- TypeScript
- Tailwind CSS

## Development Setup

1. **Install dependencies**
   ```bash
   npm install
   ```

2. **Run the development server**
   ```bash
   npm run dev
   ```

3. **Open the dashboard**
   Navigate to [http://localhost:3000](http://localhost:3000) in your browser.

## Backend Connection

Currently, the dashboard connects to mock API endpoints for system status and experiment triggering. The `LiveLogPanel` component attempts to establish a WebSocket connection to `ws://localhost:9091` to stream logs. If the Havoc Engine backend is not running, the dashboard gracefully handles connection failures and displays a "disconnected" state.

*(Future implementation phases will wire these endpoints to the real Havoc Engine API.)*
