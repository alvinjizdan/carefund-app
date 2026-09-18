import { Container, Section, PageHeader } from "@/components/layout";
import { Alert } from "@/components/feedback";

export default function UnauthorizedPage() {
  return (
    <Section spacing="lg">
      <Container size="md">
        <PageHeader
          title="Access Restricted"
          description="You do not have the required permissions to access this administrative resource."
        />
        <div className="mt-6">
          <Alert
            variant="warning"
            title="Authorization Required"
          >
            Your current account roles are not authorized to view or manage this area.
            If you believe this is an error, please contact the system administrator.
          </Alert>
        </div>
      </Container>
    </Section>
  );
}
