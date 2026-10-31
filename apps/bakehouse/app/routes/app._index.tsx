import type { HeadersFunction, LoaderFunctionArgs } from "react-router";
import { boundary } from "@shopify/shopify-app-react-router/server";
import { authenticate } from "../shopify.server";

export const loader = async ({ request }: LoaderFunctionArgs) => {
  await authenticate.admin(request);
  return null;
};

// "This week" home: next bake status, orders vs. loaf limit, needs-attention cards (M4).
export default function ThisWeek() {
  return (
    <s-page heading="This week">
      <s-section heading="Your next bake">
        <s-paragraph>
          Your bakes will appear here once your baking schedule is set up.
        </s-paragraph>
      </s-section>
    </s-page>
  );
}

export const headers: HeadersFunction = (headersArgs) => {
  return boundary.headers(headersArgs);
};
