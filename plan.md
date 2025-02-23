# Pulse – Proxmox Monitoring Dashboard

## I. Core Features

### Dashboard Grid:
- **Metrics Display:**  
  - Show Name, CPU, Memory, and Network metrics using horizontal bar graphs.
  - Update metrics in real-time every 2 seconds via a WebSocket connection.
- **Visual Feedback:**  
  - Include loading states and visual indicators for connection issues.
- **Performance:**  
  - Optimize rendering using virtualization techniques for handling large datasets.

### Interactive Controls:
- **Sorting:**  
  - Allow column sorting via header clicks (toggle between sorting on/off).
- **Filtering:**  
  - Use live CPU, Memory, and Network threshold sliders for filtering rows.
  - Integrate a search box that supports multi-filter bubbles.
- **Global Reset:**  
  - Implement an ESC key function to reset all filters and sorting with clear visual feedback.
- **UX Enhancements:**  
  - Provide intuitive, real-time updates and transitions for every user interaction.

---

## II. Technical Stack

### Frontend
- **Framework:** React with TypeScript  
- **Styling:** TailwindCSS  
- **Real-Time:** Socket.IO client for WebSocket updates  
- **State Management:** React Context and React Query (for data fetching and caching)  
- **Optimization:** Use React Window for list virtualization

### Backend
- **Runtime:** Node.js with TypeScript  
- **Framework:** Express.js for REST API endpoints  
- **Real-Time:** Socket.IO for WebSocket connections  
- **Security:**  
  - Secure credential storage and encrypted configurations
  - Input validation and sanitization for all endpoints
- **Logging & Caching:**  
  - Utilize logging (e.g., Winston) and caching (e.g., Node-Cache) for improved performance

---

## III. Project Structure

```plaintext
pulse/
├── client/                    # React frontend
│   ├── public/
│   ├── src/
│   │   ├── assets/            # Images, fonts, etc.
│   │   ├── components/        # Reusable UI components (Grid, Sliders, SearchBox)
│   │   ├── hooks/             # Custom hooks (useWebSocket, useSorting)
│   │   ├── pages/             # Main pages (Dashboard)
│   │   ├── services/          # API & WebSocket service calls
│   │   ├── styles/            # Tailwind and global styles
│   │   └── App.tsx            # Application root
├── server/                    # Node.js backend
│   ├── src/
│   │   ├── config/            # Environment and security configurations
│   │   ├── controllers/       # API request handlers
│   │   ├── middlewares/       # Error handling, authentication, and input validation
│   │   ├── routes/            # API endpoints
│   │   ├── services/          # Business logic (Proxmox integration, credential management)
│   │   └── server.ts          # Express & Socket.IO server setup
├── docker/                    # Container setup
│   ├── Dockerfile             # Dockerfile for building the app
│   └── docker-compose.yml     # Compose file for local development and deployment
├── tests/                     # Unit, integration, and E2E tests
├── .gitignore                 # Ignored files (node_modules, env, logs)
└── README.md                  # Project documentation and guidelines
```

---

## IV. Implementation Plan

### Phase 1: Project Setup
- **Repository & Structure:**  
  - Initialize the repository with the outlined structure.
  - Set up Git with a proper `.gitignore` for Node.js and React.
- **Development Environment:**  
  - Configure basic React and Node.js applications.
  - Set up Docker for both development and production environments.
- **Code Quality:**  
  - Integrate ESLint, Prettier, and commit guidelines.
- **CI/CD:**  
  - Set up initial CI/CD pipelines (e.g., GitHub Actions) for automated testing and builds.

### Phase 2: Core Feature Development

1. **Dashboard Grid:**
   - Develop the real-time metrics display using WebSocket (Socket.IO).
   - Render horizontal bar graphs for each metric.
   - Optimize performance with list virtualization and batch updates.
   - Create error handling and loading states for better UX.
   - Write tests for data rendering and WebSocket connectivity.

3. **Interactive Features:**
   - Implement column sorting with toggle functionality.
   - Build live filtering controls with threshold sliders and search box with filter bubbles.
   - Develop the global reset functionality (ESC key) with clear visual feedback.
   - Ensure robust testing for all interactive components and state resets.

### Phase 3: Polish, Security & Deployment

- **Security Enhancements:**
  - Ensure all credentials and sensitive data are stored securely.
  - Validate and sanitize all inputs and WebSocket messages.
  - Document and implement best practices for authentication and rate limiting.
- **Testing & Performance:**
  - Finalize comprehensive unit, integration, and E2E tests.
  - Optimize performance on both frontend (rendering, caching) and backend (API, WebSocket) sides.
- **Documentation & CI/CD:**
  - Update README, contribution guidelines, and API documentation.
  - Refine CI/CD pipelines for automated vulnerability scanning and deployments.
- **Docker Deployment:**
  - Prepare Dockerfiles for production and provide a clear docker-compose guide.
  - Write deployment instructions for various environments.

---

## V. Final Considerations

- **Continuous Integration:**  
  - Regularly run automated tests on each commit.
  - Integrate performance and vulnerability scanning into your CI/CD pipeline.

- **User Experience:**  
  - Design graceful error states and clear user feedback.
  - Ensure the ESC key reset function is discoverable and provides immediate visual confirmation.

- **Documentation & Maintenance:**  
  - Keep comprehensive, up-to-date documentation as the project evolves.
  - Adopt a modular approach for scalability and future feature enhancements.

- **Security:**  
  - Prioritize secure handling of credentials.
  - Plan for potential future authentication and authorization requirements.

---
