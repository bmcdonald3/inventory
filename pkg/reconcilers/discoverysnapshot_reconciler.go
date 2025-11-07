// pkg/reconcilers/discoverysnapshot_reconciler.go
package reconcilers

import (
	"context"
	"fmt"
	"time"

	"github.com/openchami/fabrica/pkg/reconcile"
	"github.com/openchami/fabrica/pkg/storage"
	
	// Import your resource definition
	"github.com/user/inventory-api/pkg/resources/discoverysnapshot"
)

// DiscoverySnapshotReconciler reconciles a DiscoverySnapshot resource
type DiscoverySnapshotReconciler struct {
	reconcile.BaseReconciler
	Storage storage.Storage
}

// GetResourceKind returns the resource kind "DiscoverySnapshot"
func (r *DiscoverySnapshotReconciler) GetResourceKind() string {
	return discoverysnapshot.Kind // "DiscoverySnapshot"
}

// Reconcile is the core logic. It's triggered when a DiscoverySnapshot is created or updated.
func (r *DiscoverySnapshotReconciler) Reconcile(ctx context.Context, resource interface{}) (reconcile.Result, error) {
	// Cast the resource to our specific type
	snapshot, ok := resource.(*discoverysnapshot.DiscoverySnapshot)
	if !ok {
		return reconcile.Result{}, fmt.Errorf("invalid resource type, expected *DiscoverySnapshot, got %T", resource)
	}

	// Use the logger from BaseReconciler
	r.Logger.Infof("Reconciling DiscoverySnapshot %s (Phase: %s)", snapshot.GetUID(), snapshot.Status.Phase)

	// --- IDEMPOTENCY CHECK ---
	// If it's already "Complete" or "Error", don't do anything.
	if snapshot.Status.Phase == "Complete" || snapshot.Status.Phase == "Error" {
		r.Logger.Infof("Snapshot %s already processed. Skipping.", snapshot.GetUID())
		return reconcile.Result{}, nil
	}

	// --- START PROCESSING ---
	// Update phase to "Processing"
	snapshot.Status.Phase = "Processing"
	snapshot.Status.Message = "Reconciliation started."
	if err := r.Storage.Save(ctx, snapshot.GetKind(), snapshot.GetUID(), snapshot); err != nil {
		r.Logger.Errorf("Failed to update status to Processing for %s: %v", snapshot.GetUID(), err)
		return reconcile.Result{}, err // Return error for retry
	}

	//
	// --- TODO: OUR CORE LOGIC GOES HERE ---
	// 1. Unmarshal `snapshot.Spec.RawData`
	// 2. Loop through devices in the raw data
	// 3. Use `r.Storage` (or a full client) to Create/Update Device resources
	// 4. Log successes/failures to `snapshot.Status.Logs`
	//
	
	// For now, we'll just log and set it to "Complete"
	r.Logger.Infof("TODO: Implement snapshot processing logic for %s", snapshot.GetUID())
	snapshot.Status.Logs = append(snapshot.Status.Logs, "Snapshot processed successfully (stub).")

	// --- FINISH PROCESSING ---
	snapshot.Status.Phase = "Complete"
	snapshot.Status.Message = "Snapshot processed successfully."
	if err := r.Storage.Save(ctx, snapshot.GetKind(), snapshot.GetUID(), snapshot); err != nil {
		r.Logger.Errorf("Failed to update status to Complete for %s: %v", snapshot.GetUID(), err)
		return reconcile.Result{}, err
	}

	r.Logger.Infof("Successfully reconciled DiscoverySnapshot %s", snapshot.GetUID())
	
	// We are done, no need to requeue
	return reconcile.Result{}, nil
}