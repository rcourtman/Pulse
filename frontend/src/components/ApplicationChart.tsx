"use client";

import React, { useState } from "react";
import { Pie } from "react-chartjs-2";
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from "chart.js";
import ChartDataLabels from "chartjs-plugin-datalabels";
import Modal from "@/components/Modal";

ChartJS.register(ArcElement, Tooltip, Legend, ChartDataLabels);

interface ApplicationChartProps {
  data: { nsapp: string }[];
}

const ApplicationChart: React.FC<ApplicationChartProps> = ({ data }) => {
  const [isChartOpen, setIsChartOpen] = useState(false);
  const [isTableOpen, setIsTableOpen] = useState(false);
  const [chartStartIndex, setChartStartIndex] = useState(0);

  const appCounts: Record<string, number> = {};
  data.forEach((item) => {
    appCounts[item.nsapp] = (appCounts[item.nsapp] || 0) + 1;
  });

  const sortedApps = Object.entries(appCounts).sort(([, a], [, b]) => b - a);
  const chartApps = sortedApps.slice(chartStartIndex, chartStartIndex + 20);

  const chartData = {
    labels: chartApps.map(([name]) => name),
    datasets: [
      {
        label: "Applications",
        data: chartApps.map(([, count]) => count),
        backgroundColor: [
          "#ff6384",
          "#36a2eb",
          "#ffce56",
          "#4bc0c0",
          "#9966ff",
          "#ff9f40",
        ],
      },
    ],
  };

  return (
    <div className="mt-6 text-center">
      <button
        onClick={() => setIsChartOpen(true)}
        className="m-2 p-2 bg-blue-500 text-white rounded"
      >
        📊 Open Chart
      </button>
      <button
        onClick={() => setIsTableOpen(true)}
        className="m-2 p-2 bg-green-500 text-white rounded"
      >
        📋 Open Table
      </button>

      <Modal isOpen={isChartOpen} onClose={() => setIsChartOpen(false)}>
        <h2 className="text-xl font-bold mb-4">Top Applications (Chart)</h2>
        <div className="w-3/4 mx-auto">
          <Pie
            data={chartData}
            options={{
              plugins: {
                legend: { display: false },
                datalabels: {
                  color: "white",
                  font: { weight: "bold" },
                  formatter: (value, context) =>
                    context.chart.data.labels?.[context.dataIndex] || "",
                },
              },
            }}
          />
        </div>
        <div className="flex justify-center space-x-4 mt-4">
          <button
            onClick={() => setChartStartIndex(Math.max(0, chartStartIndex - 20))}
            disabled={chartStartIndex === 0}
            className="p-2 border rounded bg-blue-500 text-white"
          >
            ◀ Vorherige 20
          </button>
          <button
            onClick={() => setChartStartIndex(chartStartIndex + 20)}
            disabled={chartStartIndex + 20 >= sortedApps.length}
            className="p-2 border rounded bg-blue-500 text-white"
          >
            Nächste 20 ▶
          </button>
        </div>
      </Modal>

      <Modal isOpen={isTableOpen} onClose={() => setIsTableOpen(false)}>
        <h2 className="text-xl font-bold mb-4">Application Count Table</h2>
        <table className="w-full border-collapse border border-gray-600">
          <thead>
            <tr className="bg-gray-800 text-white">
              <th className="p-2 border">Application</th>
              <th className="p-2 border">Count</th>
            </tr>
          </thead>
          <tbody>
            {sortedApps.map(([name, count]) => (
              <tr key={name} className="hover:bg-gray-200">
                <td className="p-2 border">{name}</td>
                <td className="p-2 border">{count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Modal>
    </div>
  );
};

export default ApplicationChart;
