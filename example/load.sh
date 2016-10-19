while true; do
  kubectl scale deployment nginx --replicas=0
  kubectl scale deployment nginx --replicas=1
  kubectl scale deployment nginx --replicas=10
  kubectl scale deployment nginx --replicas=24
  kubectl scale deployment nginx --replicas=3
  kubectl scale deployment nginx --replicas=5
  kubectl scale deployment nginx --replicas=8
  kubectl scale deployment nginx --replicas=1
  kubectl scale deployment nginx --replicas=0
  kubectl scale deployment nginx --replicas=10
done
