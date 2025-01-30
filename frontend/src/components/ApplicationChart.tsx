"use client";

import React, { useState } from "react";
import { Pie } from "react-chartjs-2";
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from "chart.js";

ChartJS.register(ArcElement, Tooltip, Legend);

interface ApplicationChartProps {
  data: { nsapp: string }[];
}

const ApplicationChart: React.FC<ApplicationChartProps> = ({ data }) => {
  const [visibleCount, setVisibleCount] = useState(20); // Zeigt zuerst 20 an
  const [highlighted, setHighlighted] = useState<string | null>(null);
  const [chartStartIndex, setChartStartIndex] = useState(0);

  const appCounts: Record<string, number> = {};
  data.forEach((item) => {
    appCounts[item.nsapp] = (appCounts[item.nsapp] || 0) + 1;
  });

  const sortedApps = Object.entries(appCounts)
    .sort(([, a], [, b]) => b - a)
    .slice(0, visibleCount);

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
    <div className="mt-6">
      <table className="w-full border-collapse border border-gray-600">
        <thead>
          <tr className="bg-gray-800 text-white">
            <th className="p-2 border">Application</th>
            <th className="p-2 border">Count</th>
          </tr>
        </thead>
        <tbody>
          {sortedApps.map(([name, count]) => (
            <tr
              key={name}
              className={`cursor-pointer ${highlighted === name ? "bg-yellow-300" : "hover:bg-gray-200"}`}
              onMouseEnter={() => setHighlighted(name)}
              onMouseLeave={() => setHighlighted(null)}
            >
              <td className="p-2 border">{name}</td>
              <td className="p-2 border">{count}</td>
            </tr>
          ))}
        </tbody>
      </table>

      {visibleCount < Object.keys(appCounts).length && (
        <button
          onClick={() => setVisibleCount((prev) => prev + 20)}
          className="mt-4 p-2 bg-blue-500 text-white rounded"
        >
          Load More
        </button>
      )}

      <div className="w-1/2 mx-auto my-6">
        <Pie data={chartData} />
      </div>

      <div className="flex justify-center space-x-4">
        <button
          onClick={() => setChartStartIndex(Math.max(0, chartStartIndex - 20))}
          disabled={chartStartIndex === 0}
          className={`p-2 border rounded ${chartStartIndex === 0 ? "bg-gray-400 cursor-not-allowed" : "bg-blue-500 text-white"}`}
        >
          ◀ Vorherige 20
        </button>
        <button
          onClick={() => setChartStartIndex(chartStartIndex + 20)}
          disabled={chartStartIndex + 20 >= sortedApps.length}
          className={`p-2 border rounded ${chartStartIndex + 20 >= sortedApps.length ? "bg-gray-400 cursor-not-allowed" : "bg-blue-500 text-white"}`}
        >
          Next 20 ▶
        </button>
      </div>
    </div>
  );
};

export default ApplicationChart;
