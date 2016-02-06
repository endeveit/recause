Vagrant.configure(2) do |config|
  config.vm.box = "elastic/debian-8-x86_64"
  config.vm.provision :shell, path: "data/vagrant-bootstrap.sh"
  config.vm.network "forwarded_port", guest: 9200, host: 9200
  # config.vm.network "private_network", ip: "192.168.33.10"
  config.vm.provider "virtualbox" do |vb|
    vb.name = "recause-es"
    vb.memory = 512
    vb.cpus = 1
  end
end
