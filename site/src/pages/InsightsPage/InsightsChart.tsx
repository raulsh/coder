import "chartjs-adapter-date-fns";
import { useTheme } from "@emotion/react"
import {
  CategoryScale,
  Chart as ChartJS,
  type ChartOptions,
  defaults,
  Filler,
  Legend,
  LinearScale,
  LineElement,
  TimeScale,
  Title,
  Tooltip,
  PointElement,
} from "chart.js";
import annotationPlugin from "chartjs-plugin-annotation";
import dayjs from "dayjs";
import { useMemo, type FC } from "react";
import { Line } from "react-chartjs-2";

ChartJS.register(
  CategoryScale,
  LinearScale,
  TimeScale,
  LineElement,
  PointElement,
  Filler,
  Title,
  Tooltip,
  Legend,
  annotationPlugin,
);

export interface InsightsChartProps {
  className?: string
  lines: Array<{
    label: string
    pointBackgroundColor: string
    pointBorderColor: string
    borderColor: string
    data: Array<{ date: string; amount: number }>
  }>;
  interval: "day" | "week";
}

const InsightsChart: FC<InsightsChartProps> = ({
  className,
  lines,
  interval,
}) => {
  const theme = useTheme()
  const labels = useMemo(() => {
    return lines.flatMap((line) => line.data.map((val) => dayjs(val.date).format("YYYY-MM-DD")))
  }, [lines])

  defaults.font.family = theme.typography.fontFamily as string;
  defaults.color = theme.palette.text.secondary;

  const options: ChartOptions<"line"> = {
    responsive: true,
    animation: false,
    plugins: {
      legend: {
        display: true,
      },
      tooltip: {
        displayColors: false,
        callbacks: {
          title: (context) => {
            const date = new Date(context[0].parsed.x);
            return date.toLocaleDateString();
          },
        },
      },
    },
    scales: {
      y: {
        grid: { color: theme.palette.divider },
        suggestedMin: 0,
        ticks: {
          precision: 0,
          format: {
            style: "percent",
          },
          // format: (n) => `hi${}`
        },
      },

      x: {
        grid: { color: theme.palette.divider },
        ticks: {
          // stepSize: data.length > 10 ? 2 : undefined,
        },
        type: "time",
        time: {
          unit: interval,
        },
      },
    },
    maintainAspectRatio: false,
  };

  return (
    <Line
      className={className}
      data-chromatic="ignore"
      data={{
        labels: labels,
        datasets: lines.map((line) => ({
          label: line.label,
          data: line.data.map((val) => val.amount),
          pointBackgroundColor: line.pointBackgroundColor,
          pointBorderColor: line.pointBorderColor,
          borderColor: line.borderColor,
        })),
      }}
      options={options}
    />
  );
}

export default InsightsChart
