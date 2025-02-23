interface Node {
    id: string;
    host: string;
    tokenId: string;
    tokenSecret: string;
    nodeName: string;
}

export class NodeMonitor {
    private nodes: Node[] = [];

    getNode(nodeId: string) {
        return this.nodes.find(node => node.id === nodeId);
    }

    addNode(node: Node) {
        // Check for existing node with same host
        const existingNode = this.nodes.find(n => n.host === node.host);
        if (existingNode) {
            // Update existing node instead of adding new one
            Object.assign(existingNode, node);
            return;
        }
        this.nodes.push(node);
    }

    removeNode(nodeId: string) {
        this.nodes = this.nodes.filter(node => node.id !== nodeId);
    }

    getNodes() {
        return this.nodes;
    }
}

// Create and export a singleton instance
export const monitor = new NodeMonitor(); 