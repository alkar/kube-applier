// +build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Waybill) DeepCopyInto(out *Waybill) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Waybill.
func (in *Waybill) DeepCopy() *Waybill {
	if in == nil {
		return nil
	}
	out := new(Waybill)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Waybill) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WaybillList) DeepCopyInto(out *WaybillList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Waybill, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WaybillList.
func (in *WaybillList) DeepCopy() *WaybillList {
	if in == nil {
		return nil
	}
	out := new(WaybillList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *WaybillList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WaybillSpec) DeepCopyInto(out *WaybillSpec) {
	*out = *in
	if in.AutoApply != nil {
		in, out := &in.AutoApply, &out.AutoApply
		*out = new(bool)
		**out = **in
	}
	if in.DelegateServiceAccountSecretRef != nil {
		in, out := &in.DelegateServiceAccountSecretRef, &out.DelegateServiceAccountSecretRef
		*out = new(string)
		**out = **in
	}
	if in.Prune != nil {
		in, out := &in.Prune, &out.Prune
		*out = new(bool)
		**out = **in
	}
	if in.PruneBlacklist != nil {
		in, out := &in.PruneBlacklist, &out.PruneBlacklist
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.RepositoryPath != nil {
		in, out := &in.RepositoryPath, &out.RepositoryPath
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WaybillSpec.
func (in *WaybillSpec) DeepCopy() *WaybillSpec {
	if in == nil {
		return nil
	}
	out := new(WaybillSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WaybillStatus) DeepCopyInto(out *WaybillStatus) {
	*out = *in
	if in.LastRun != nil {
		in, out := &in.LastRun, &out.LastRun
		*out = new(WaybillStatusRun)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WaybillStatus.
func (in *WaybillStatus) DeepCopy() *WaybillStatus {
	if in == nil {
		return nil
	}
	out := new(WaybillStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WaybillStatusRun) DeepCopyInto(out *WaybillStatusRun) {
	*out = *in
	in.Finished.DeepCopyInto(&out.Finished)
	in.Started.DeepCopyInto(&out.Started)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WaybillStatusRun.
func (in *WaybillStatusRun) DeepCopy() *WaybillStatusRun {
	if in == nil {
		return nil
	}
	out := new(WaybillStatusRun)
	in.DeepCopyInto(out)
	return out
}
