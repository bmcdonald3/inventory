// pkg/reconcilers/discoverysnapshot_reconciler.go
package reconcilers

import (
	"context"
	"encoding/json"
	"fmt"
	"time" // <<< FIX: Import the time package

	"github.com/openchami/fabrica/pkg/reconcile"
	"github.com/openchami/fabrica/pkg/storage"

	// Import your resource definition
	"github.com/user/inventory-api/pkg/resources/discoverysnapshot"
)

// DiscoverySnapshotReconciler reconciles a DiscoverySnapshot resource
type DiscoverySnapshotReconciler struct {
	reconcile.BaseReconciler
	Storage storage.StorageBackend
}

// GetResourceKind returns the resource kind "DiscoverySnapshot"
func (r *DiscoverySnapshotReconciler) GetResourceKind() string {
	// Use the exact string from the resource's GetKind() method
	return "DiscoverySnapshot"
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
	if snapshot.Status.Phase == "Complete" || snapshot.Status.Phase == "Error" {
		r.Logger.Infof("Snapshot %s already processed. Skipping.", snapshot.GetUID())
		return reconcile.Result{}, nil
	}

	// <<< FIX: Handle the initial "create" event race condition >>>
	// If the phase is empty, it means the HTTP handler just created it.
	// Set it to "Pending" and requeue to let the HTTP handler finish.
	if snapshot.Status.Phase == "" {
		r.Logger.Infof("New snapshot %s detected, setting to Pending and requeueing.", snapshot.GetUID())
		snapshot.Status.Phase = "Pending"
		snapshot.Status.Message = "Snapshot queued for processing."
		snapshotData, err := json.Marshal(snapshot)
		if err != nil {
			r.Logger.Errorf("Failed to marshal snapshot %s for Pending update: %v", snapshot.GetUID(), err)
			return reconcile.Result{}, err
		}
		if err := r.Storage.Save(ctx, r.GetResourceKind(), snapshot.GetUID(), snapshotData); err != nil {
			r.Logger.Errorf("Failed to update status to Pending for %s: %v", snapshot.GetUID(), err)
			return reconcile.Result{}, err // Return error, will retry
		}
		// Requeue with a small delay
		return reconcile.Result{Requeue: true, RequeueAfter: 1 * time.Second}, nil
	}
	// <<< END FIX >>>

	// --- START PROCESSING (Phase will be "Pending" here) ---
	snapshot.Status.Phase = "Processing"
	snapshot.Status.Message = "Reconciliation started."

	snapshotData, err := json.Marshal(snapshot)
	if err != nil {
		r.Logger.Errorf("Failed to marshal snapshot %s for saving: %v", snapshot.GetUID(), err)
		return reconcile.Result{}, err // Return error, will retry
	}
	if err := r.Storage.Save(ctx, r.GetResourceKind(), snapshot.GetUID(), snapshotData); err != nil {
		r.Logger.Errorf("Failed to update status to Processing for %s: %v", snapshot.GetUID(), err)
		return reconcile.Result{}, err // Return error for retry
	}

	//
	// --- TODO: OUR CORE LOGIC GOES HERE ---
	//
	// 1. Unmarshal `snapshot.Spec.RawData`
	// 2. Loop through devices in the raw data
	// 3. Use `r.Storage` to Create/Update the *actual* Device resources
	//    (e.g., r.Storage.Save(ctx, "Device", newDevice.GetUID(), newDevice))
	// 4. Log successes/failures to `snapshot.Status.Logs`
	//

	// For now, we'll just log and set it to "Complete"
	r.Logger.Infof("TODO: Implement snapshot processing logic for %s", snapshot.GetUID())
	snapshot.Status.Logs = append(snapshot.Status.Logs, "Snapshot processed successfully (stub).")

	// --- FINISH PROCESSING ---
	snapshot.Status.Phase = "Complete"
	snapshot.Status.Message = "Snapshot processed successfully."

	finalSnapshotData, err := json.Marshal(snapshot)
	if err != nil {
		r.Logger.Errorf("Failed to marshal final snapshot %s for saving: %v", snapshot.GetUID(), err)
		return reconcile.Result{}, err
	}
	if err := r.Storage.Save(ctx, r.GetResourceKind(), snapshot.GetUID(), finalSnapshotData); err != nil {
		r.Logger.Errorf("Failed to update status to Complete for %s: %v", snapshot.GetUID(), err)
		return reconcile.Result{}, err
	}

	r.Logger.Infof("Successfully reconciled DiscoverySnapshot %s", snapshot.GetUID())

	// We are done, no need to requeue
	return reconcile.Result{}, nil
}