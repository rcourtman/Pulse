"use client";

import React, { useState } from "react";
import { Pie } from "react-chartjs-2";
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from "chart.js";
import ChartDataLabels from "chartjs-plugin-datalabels"; 

ChartJS.register(ArcElement, Tooltip, Legend, ChartDataLabels);

interface ApplicationChartProps {
  data: { nsapp: string }[];
}

const ApplicationChart: React.FC<ApplicationChartProps> = ({ data }) => {
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
      <div className="w-1/2 mx-auto my-6">
        <Pie
          data={chartData}
          options={{
            plugins: {
              legend: { display: false },
              datalabels: {
                color: "white",
                font: { weight: "bold" },
                formatter: (value, context) => {
                  return context.chart.data.labels?.[context.dataIndex] || "";
                },
              },
            },
          }}
        />
      </div>

      <div className="flex justify-center space-x-4">
        <button
          onClick={() => setChartStartIndex(Math.max(0, chartStartIndex - 20))}
          disabled={chartStartIndex === 0}
          className={`p-2 border rounded ${chartStartIndex === 0 ? "bg-gray-400 cursor-not-allowed" : "bg-blue-500 text-white"}`}
        >
          ◀ Last 20
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
